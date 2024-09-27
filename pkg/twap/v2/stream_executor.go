package twap

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var defaultUpdateInterval = time.Minute

// FixedQuantityExecutor is a TWAP executor that places orders on the exchange using the exchange's stream API.
// It uses a fixed target quantity to place orders.
type FixedQuantityExecutor struct {
	exchange types.Exchange

	// configuration fields

	symbol                        string
	side                          types.SideType
	targetQuantity, sliceQuantity fixedpoint.Value

	// updateInterval is a fixed update interval for placing new order
	updateInterval time.Duration

	// delayInterval is the delay interval between each order placement
	delayInterval time.Duration

	// numOfTicks is the number of price ticks behind the best bid to place the order
	numOfTicks int

	// stopPrice is the price limit for the order
	// for buy-orders, the price limit is the maximum price
	// for sell-orders, the price limit is the minimum price
	stopPrice fixedpoint.Value

	// deadlineTime is the deadline time for the order execution
	deadlineTime *time.Time

	executionCtx    context.Context
	cancelExecution context.CancelFunc

	userDataStreamCtx    context.Context
	cancelUserDataStream context.CancelFunc

	market           types.Market
	marketDataStream types.Stream

	orderBook *types.StreamOrderBook

	userDataStream types.Stream

	orderUpdateRateLimit *rate.Limiter
	activeMakerOrders    *bbgo.ActiveOrderBook
	orderStore           *core.OrderStore
	position             *types.Position
	tradeCollector       *core.TradeCollector

	logger logrus.FieldLogger

	mu sync.Mutex

	userDataStreamConnectC   chan struct{}
	marketDataStreamConnectC chan struct{}
	done                     *DoneSignal
}

func NewFixedQuantityExecutor(
	exchange types.Exchange,
	symbol string,
	market types.Market,
	side types.SideType,
	targetQuantity, sliceQuantity fixedpoint.Value,
) *FixedQuantityExecutor {

	marketDataStream := exchange.NewStream()
	marketDataStream.SetPublicOnly()
	marketDataStream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{
		Depth: types.DepthLevelMedium,
	})

	orderBook := types.NewStreamBook(symbol, exchange.Name())
	orderBook.BindStream(marketDataStream)

	userDataStream := exchange.NewStream()
	orderStore := core.NewOrderStore(symbol)
	position := types.NewPositionFromMarket(market)
	tradeCollector := core.NewTradeCollector(symbol, position, orderStore)
	orderStore.BindStream(userDataStream)

	activeMakerOrders := bbgo.NewActiveOrderBook(symbol)

	e := &FixedQuantityExecutor{
		exchange:       exchange,
		symbol:         symbol,
		side:           side,
		market:         market,
		targetQuantity: targetQuantity,
		sliceQuantity:  sliceQuantity,
		updateInterval: defaultUpdateInterval,
		logger: logrus.WithFields(logrus.Fields{
			"executor": "twapStream",
			"symbol":   symbol,
		}),

		marketDataStream: marketDataStream,
		orderBook:        orderBook,

		userDataStream: userDataStream,

		activeMakerOrders: activeMakerOrders,
		orderStore:        orderStore,
		tradeCollector:    tradeCollector,
		position:          position,
		done:              NewDoneSignal(),

		userDataStreamConnectC:   make(chan struct{}),
		marketDataStreamConnectC: make(chan struct{}),
	}

	e.tradeCollector.OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		e.logger.Info(trade.String())
	})
	e.tradeCollector.BindStream(e.userDataStream)

	activeMakerOrders.OnFilled(e.handleFilledOrder)
	activeMakerOrders.BindStream(e.userDataStream)

	e.marketDataStream.OnConnect(func() {
		e.logger.Info("market data stream on connect")
		close(e.marketDataStreamConnectC)
		e.logger.Infof("marketDataStreamConnectC closed")
	})

	// private channels
	e.userDataStream.OnAuth(func() {
		e.logger.Info("user data stream on auth")
		close(e.userDataStreamConnectC)
		e.logger.Info("userDataStreamConnectC closed")
	})

	return e
}

func (e *FixedQuantityExecutor) SetDeadlineTime(t time.Time) {
	e.deadlineTime = &t
}

func (e *FixedQuantityExecutor) SetDelayInterval(delayInterval time.Duration) {
	e.delayInterval = delayInterval
}

func (e *FixedQuantityExecutor) SetUpdateInterval(updateInterval time.Duration) {
	e.updateInterval = updateInterval
}

func (e *FixedQuantityExecutor) SetNumOfTicks(numOfTicks int) {
	e.numOfTicks = numOfTicks
}

func (e *FixedQuantityExecutor) SetStopPrice(price fixedpoint.Value) {
	e.stopPrice = price
}

func (e *FixedQuantityExecutor) connectMarketData(ctx context.Context) {
	e.logger.Infof("connecting market data stream...")
	if err := e.marketDataStream.Connect(ctx); err != nil {
		e.logger.WithError(err).Errorf("market data stream connect error")
	}
}

func (e *FixedQuantityExecutor) connectUserData(ctx context.Context) {
	e.logger.Infof("connecting user data stream...")
	if err := e.userDataStream.Connect(ctx); err != nil {
		e.logger.WithError(err).Errorf("user data stream connect error")
	}
}

func (e *FixedQuantityExecutor) handleFilledOrder(order types.Order) {
	e.logger.Info(order.String())

	// filled event triggers the order removal from the active order store
	// we need to ensure we received every order update event before the execution is done.
	e.cancelContextIfTargetQuantityFilled()
}

func (e *FixedQuantityExecutor) cancelContextIfTargetQuantityFilled() bool {
	// ensure that the trades are processed
	e.tradeCollector.Process()

	// now get the base quantity from the position
	base := e.position.GetBase()

	if base.Abs().Sub(e.targetQuantity).Compare(e.market.MinQuantity.Neg()) >= 0 {
		e.logger.Infof("position is filled with target quantity, canceling the order execution context")
		e.cancelExecution()
		return true
	}
	return false
}

func (e *FixedQuantityExecutor) SetOrderUpdateRateLimit(rateLimit *rate.Limiter) {
	e.orderUpdateRateLimit = rateLimit
}

func (e *FixedQuantityExecutor) cancelActiveOrders(ctx context.Context) error {
	gracefulCtx, gracefulCancel := context.WithTimeout(ctx, 30*time.Second)
	defer gracefulCancel()
	return e.activeMakerOrders.GracefulCancel(gracefulCtx, e.exchange)
}

func (e *FixedQuantityExecutor) orderUpdater(ctx context.Context) {
	defer func() {
		if err := e.cancelActiveOrders(ctx); err != nil {
			e.logger.WithError(err).Error("cancel active orders error")
		}

		e.cancelUserDataStream()
		e.done.Emit()
	}()

	ticker := time.NewTimer(e.updateInterval)
	defer ticker.Stop()

	monitor := bbgo.NewBboMonitor()

	for {
		select {
		case <-ctx.Done():
			return

		case <-e.orderBook.C:
			changed := monitor.UpdateFromBook(e.orderBook)
			if !changed {
				continue
			}

			// orderBook.C sends a signal when any price or quantity changes in the order book
			if e.cancelContextIfTargetQuantityFilled() {
				return
			}

			e.logger.Infof("%s order book changed, checking order...", e.symbol)

			if err := e.updateOrder(ctx); err != nil {
				e.logger.WithError(err).Errorf("order update failed")
			}

		case <-ticker.C:
			changed := monitor.UpdateFromBook(e.orderBook)
			if !changed {
				continue
			}

			if e.cancelContextIfTargetQuantityFilled() {
				return
			}

			if err := e.updateOrder(ctx); err != nil {
				e.logger.WithError(err).Errorf("order update failed")
			}
		}
	}
}

func (e *FixedQuantityExecutor) updateOrder(ctx context.Context) error {
	if e.orderUpdateRateLimit != nil && !e.orderUpdateRateLimit.Allow() {
		e.logger.Infof("rate limit exceeded, skip updating order")
		return nil
	}

	book := e.orderBook.Copy()
	sideBook := book.SideBook(e.side)

	first, ok := sideBook.First()
	if !ok {
		return fmt.Errorf("empty %s %s side book", e.symbol, e.side)
	}

	// if there is no gap between the first price entry and the second price entry
	second, ok := sideBook.Second()
	if !ok {
		return fmt.Errorf("no secoond price on the %s order book %s, can not update", e.symbol, e.side)
	}

	tickSize := e.market.TickSize
	numOfTicks := fixedpoint.NewFromInt(int64(e.numOfTicks))
	tickSpread := tickSize.Mul(numOfTicks)

	// check and see if we need to cancel the existing active orders
	for e.activeMakerOrders.NumOfOrders() > 0 {
		orders := e.activeMakerOrders.Orders()
		if len(orders) > 1 {
			e.logger.Warnf("found more than 1 %s open orders on the orderbook", e.symbol)
		}

		// get the first active order
		order := orders[0]
		orderPrice := order.Price
		// quantity := fixedpoint.NewFromFloat(order.Quantity)

		remainingQuantity := order.Quantity.Sub(order.ExecutedQuantity)
		if remainingQuantity.Compare(e.market.MinQuantity) <= 0 {
			logrus.Infof("order remaining quantity %s is less than the market minimal quantity %s, skip updating order", remainingQuantity.String(), e.market.MinQuantity.String())
			return nil
		}

		// if the first bid price or first ask price is the same to the current active order
		// we should skip updating the order
		// DO NOT UPDATE IF:
		//   tickSpread > 0 AND current order price == second price + tickSpread
		//   current order price == first price
		logrus.Infof("orderPrice = %s, best price = %s, second level price = %s, tickSpread = %s",
			orderPrice.String(),
			first.Price.String(),
			second.Price.String(),
			tickSpread.String())

		switch e.side {
		case types.SideTypeBuy:
			if tickSpread.Sign() > 0 && orderPrice.Compare(second.Price.Add(tickSpread)) == 0 {
				e.logger.Infof("the current order is already on the best ask price %s, skip update", orderPrice.String())
				return nil
			} else if orderPrice == first.Price {
				e.logger.Infof("the current order is already on the best bid price %s, skip update", orderPrice.String())
				return nil
			}

		case types.SideTypeSell:
			if tickSpread.Sign() > 0 && orderPrice.Compare(second.Price.Sub(tickSpread)) == 0 {
				e.logger.Infof("the current order is already on the best ask price %s, skip update", orderPrice.String())
				return nil
			} else if orderPrice == first.Price {
				e.logger.Infof("the current order is already on the best ask price %s, skip update", orderPrice.String())
				return nil
			}
		}

		if err := e.cancelActiveOrders(ctx); err != nil {
			e.logger.Warnf("cancel active orders error: %v", err)
		}
	}

	e.tradeCollector.Process()

	if e.delayInterval > 0 {
		time.Sleep(e.delayInterval)
	}

	orderForm, err := e.generateOrder()
	if err != nil {
		return err
	} else if orderForm == nil {
		return nil
	}

	return e.submitOrder(ctx, *orderForm)
}

func (e *FixedQuantityExecutor) submitOrder(ctx context.Context, orderForm types.SubmitOrder) error {
	createdOrder, err := e.exchange.SubmitOrder(ctx, orderForm)
	if err != nil {
		return err
	}

	if createdOrder != nil {
		e.orderStore.Add(*createdOrder)
		e.activeMakerOrders.Add(*createdOrder)
		e.tradeCollector.Process()
	}

	return nil
}

func (e *FixedQuantityExecutor) getNewPrice() (fixedpoint.Value, error) {
	newPrice := fixedpoint.Zero
	book := e.orderBook.Copy()
	sideBook := book.SideBook(e.side)

	first, ok := sideBook.First()
	if !ok {
		return newPrice, fmt.Errorf("empty %s %s side book", e.symbol, e.side)
	}

	newPrice = first.Price
	spread, ok := book.Spread()
	if !ok {
		return newPrice, errors.New("can not calculate spread, neither bid price or ask price exists")
	}

	tickSize := e.market.TickSize
	tickSpread := tickSize.Mul(fixedpoint.NewFromInt(int64(e.numOfTicks)))
	if spread.Compare(tickSize) > 0 {
		// there is a gap in the spread
		tickSpread = fixedpoint.Min(tickSpread, spread.Sub(tickSize))
		switch e.side {
		case types.SideTypeSell:
			newPrice = newPrice.Sub(tickSpread)
		case types.SideTypeBuy:
			newPrice = newPrice.Add(tickSpread)
		}
	}

	if e.stopPrice.Sign() > 0 {
		switch e.side {
		case types.SideTypeSell:
			if newPrice.Compare(e.stopPrice) < 0 {
				logrus.Infof("%s order price %s is lower than the stop sell price %s, setting order price to the stop sell price %s",
					e.symbol,
					newPrice.String(),
					e.stopPrice.String(),
					e.stopPrice.String())
				newPrice = e.stopPrice
			}

		case types.SideTypeBuy:
			if newPrice.Compare(e.stopPrice) > 0 {
				logrus.Infof("%s order price %s is higher than the stop buy price %s, setting order price to the stop buy price %s",
					e.symbol,
					newPrice.String(),
					e.stopPrice.String(),
					e.stopPrice.String())
				newPrice = e.stopPrice
			}
		}
	}

	return newPrice, nil
}

func (e *FixedQuantityExecutor) getRemainingQuantity() fixedpoint.Value {
	base := e.position.GetBase()
	return e.targetQuantity.Sub(base.Abs())
}

func (e *FixedQuantityExecutor) isDeadlineExceeded() bool {
	if e.deadlineTime != nil && !e.deadlineTime.IsZero() {
		return time.Since(*e.deadlineTime) > 0
	}

	return false
}

func (e *FixedQuantityExecutor) calculateNewOrderQuantity(price fixedpoint.Value) (fixedpoint.Value, error) {
	minQuantity := e.market.MinQuantity
	remainingQuantity := e.getRemainingQuantity()

	if remainingQuantity.Sign() <= 0 {
		e.cancelExecution()
		return fixedpoint.Zero, nil
	}

	if remainingQuantity.Compare(minQuantity) < 0 {
		e.logger.Warnf("can not continue placing orders, the remaining quantity %s is less than the min quantity %s", remainingQuantity.String(), minQuantity.String())

		e.cancelExecution()
		return fixedpoint.Zero, nil
	}

	// if deadline exceeded, we should return the remaining quantity
	if e.isDeadlineExceeded() {
		return remainingQuantity, nil
	}

	// when slice = 1000, if we only have 998, we should adjust our quantity to 998
	orderQuantity := fixedpoint.Min(e.sliceQuantity, remainingQuantity)

	// if the remaining quantity in the next round is not enough, we should merge the remaining quantity into this round
	// if there are rest slices
	nextRemainingQuantity := remainingQuantity.Sub(e.sliceQuantity)

	if nextRemainingQuantity.Sign() > 0 && e.market.IsDustQuantity(nextRemainingQuantity, price) {
		orderQuantity = remainingQuantity
	}

	orderQuantity = e.market.AdjustQuantityByMinNotional(orderQuantity, price)
	return orderQuantity, nil
}

func (e *FixedQuantityExecutor) generateOrder() (*types.SubmitOrder, error) {
	newPrice, err := e.getNewPrice()
	if err != nil {
		return nil, err
	}

	orderQuantity, err := e.calculateNewOrderQuantity(newPrice)
	if err != nil {
		return nil, err
	}

	balances, err := e.exchange.QueryAccountBalances(e.executionCtx)
	if err != nil {
		return nil, err
	}

	switch e.side {
	case types.SideTypeSell:
		// check base balance for sell, try to sell as more as possible
		if b, ok := balances[e.market.BaseCurrency]; ok {
			orderQuantity = fixedpoint.Min(b.Available, orderQuantity)
		}

	case types.SideTypeBuy:
		// check base balance for sell, try to sell as more as possible
		if b, ok := balances[e.market.QuoteCurrency]; ok {
			orderQuantity = e.market.AdjustQuantityByMaxAmount(orderQuantity, newPrice, b.Available)
		}
	}

	if e.isDeadlineExceeded() {
		return &types.SubmitOrder{
			Symbol:   e.symbol,
			Side:     e.side,
			Type:     types.OrderTypeMarket,
			Quantity: orderQuantity,
			Market:   e.market,
		}, nil
	}

	return &types.SubmitOrder{
		Symbol:      e.symbol,
		Side:        e.side,
		Type:        types.OrderTypeLimitMaker,
		Quantity:    orderQuantity,
		Price:       newPrice,
		Market:      e.market,
		TimeInForce: types.TimeInForceGTC,
	}, nil
}

func (e *FixedQuantityExecutor) Start(ctx context.Context) error {
	if e.executionCtx != nil {
		return errors.New("executionCtx is not nil, you can't start the executor twice")
	}

	e.executionCtx, e.cancelExecution = context.WithCancel(ctx)
	e.userDataStreamCtx, e.cancelUserDataStream = context.WithCancel(ctx)

	go e.connectMarketData(e.executionCtx)
	go e.connectUserData(e.userDataStreamCtx)

	e.logger.Infof("waiting for connections ready...")

	if err := e.WaitForConnection(ctx); err != nil {
		e.cancelExecution()
		return err
	}

	e.logger.Infof("connections ready, starting order updater...")

	go e.orderUpdater(e.executionCtx)
	return nil
}

func (e *FixedQuantityExecutor) WaitForConnection(ctx context.Context) error {
	if !selectSignalOrTimeout(ctx, e.marketDataStreamConnectC, 10*time.Second) {
		return fmt.Errorf("market data stream connection timeout")
	}

	if !selectSignalOrTimeout(ctx, e.userDataStreamConnectC, 10*time.Second) {
		return fmt.Errorf("user data stream connection timeout")
	}

	return nil
}

// Done returns a channel that emits a signal when the execution is done.
func (e *FixedQuantityExecutor) Done() <-chan struct{} {
	return e.done.Chan()
}

// Shutdown stops the execution
// If we call this method, it means the execution is still running,
// We need it to:
// 1. Stop the order updater (by using the execution context)
// 2. The order updater cancels all open orders and closes the user data stream
func (e *FixedQuantityExecutor) Shutdown(shutdownCtx context.Context) {
	e.tradeCollector.Process()

	e.mu.Lock()
	if e.cancelExecution != nil {
		e.cancelExecution()
	}
	e.mu.Unlock()

	for {
		select {

		case <-shutdownCtx.Done():
			return

		case <-e.done.Chan():
			return

		}
	}
}

func selectSignalOrTimeout(ctx context.Context, c chan struct{}, timeout time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(timeout):
		return false
	case <-c:
		return true
	}
}

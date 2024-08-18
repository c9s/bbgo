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

type DoneSignal struct {
	doneC chan struct{}
	mu    sync.Mutex
}

func NewDoneSignal() *DoneSignal {
	return &DoneSignal{
		doneC: make(chan struct{}),
	}
}

func (e *DoneSignal) Emit() {
	e.mu.Lock()
	if e.doneC == nil {
		e.doneC = make(chan struct{})
	}

	close(e.doneC)
	e.mu.Unlock()
}

// Chan returns a channel that emits a signal when the execution is done.
func (e *DoneSignal) Chan() (c <-chan struct{}) {
	// if the channel is not allocated, it means it's not started yet, we need to return a closed channel
	e.mu.Lock()
	if e.doneC == nil {
		e.doneC = make(chan struct{})
		c = e.doneC
	} else {
		c = e.doneC
	}
	e.mu.Unlock()

	return c
}

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
	orderBook        *types.StreamOrderBook

	userDataStream    types.Stream
	activeMakerOrders *bbgo.ActiveOrderBook
	orderStore        *core.OrderStore
	position          *types.Position
	tradeCollector    *core.TradeCollector

	logger logrus.FieldLogger

	mu sync.Mutex

	done *DoneSignal
}

func NewStreamExecutor(
	exchange types.Exchange,
	symbol string,
	market types.Market,
	side types.SideType,
	targetQuantity, sliceQuantity fixedpoint.Value,
) *FixedQuantityExecutor {
	orderStore := core.NewOrderStore(symbol)
	position := types.NewPositionFromMarket(market)
	tradeCollector := core.NewTradeCollector(symbol, position, orderStore)
	return &FixedQuantityExecutor{
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

		orderStore:     orderStore,
		tradeCollector: tradeCollector,
		position:       position,
		done:           NewDoneSignal(),
	}
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
	base := e.position.GetBase()

	if base.Abs().Compare(e.targetQuantity) >= 0 {
		e.logger.Infof("filled target quantity, canceling the order execution context")
		e.cancelExecution()
		return true
	}
	return false
}

func (e *FixedQuantityExecutor) cancelActiveOrders(ctx context.Context) error {
	gracefulCtx, gracefulCancel := context.WithTimeout(ctx, 30*time.Second)
	defer gracefulCancel()
	return e.activeMakerOrders.GracefulCancel(gracefulCtx, e.exchange)
}

func (e *FixedQuantityExecutor) orderUpdater(ctx context.Context) {
	updateLimiter := rate.NewLimiter(rate.Every(3*time.Second), 1)

	defer func() {
		if err := e.cancelActiveOrders(ctx); err != nil {
			e.logger.WithError(err).Error("cancel active orders error")
		}

		e.cancelUserDataStream()
		e.done.Emit()
	}()

	ticker := time.NewTimer(e.updateInterval)
	defer ticker.Stop()

	monitor := NewBboMonitor()

	for {
		select {
		case <-ctx.Done():
			return

		case <-e.orderBook.C:
			changed := monitor.OnUpdateFromBook(e.orderBook)
			if !changed {
				continue
			}

			// orderBook.C sends a signal when any price or quantity changes in the order book
			if !updateLimiter.Allow() {
				break
			}

			if e.cancelContextIfTargetQuantityFilled() {
				return
			}

			e.logger.Infof("%s order book changed, checking order...", e.symbol)

			if err := e.updateOrder(ctx); err != nil {
				e.logger.WithError(err).Errorf("order update failed")
			}

		case <-ticker.C:
			changed := monitor.OnUpdateFromBook(e.orderBook)
			if !changed {
				continue
			}

			if !updateLimiter.Allow() {
				break
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
			logrus.Warnf("more than 1 %s open orders in the strategy...", e.symbol)
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
		logrus.Infof("orderPrice = %s first.Price = %s second.Price = %s tickSpread = %s", orderPrice.String(), first.Price.String(), second.Price.String(), tickSpread.String())

		switch e.side {
		case types.SideTypeBuy:
			if tickSpread.Sign() > 0 && orderPrice == second.Price.Add(tickSpread) {
				logrus.Infof("the current order is already on the best ask price %s", orderPrice.String())
				return nil
			} else if orderPrice == first.Price {
				logrus.Infof("the current order is already on the best bid price %s", orderPrice.String())
				return nil
			}

		case types.SideTypeSell:
			if tickSpread.Sign() > 0 && orderPrice == second.Price.Sub(tickSpread) {
				logrus.Infof("the current order is already on the best ask price %s", orderPrice.String())
				return nil
			} else if orderPrice == first.Price {
				logrus.Infof("the current order is already on the best ask price %s", orderPrice.String())
				return nil
			}
		}

		if err := e.cancelActiveOrders(ctx); err != nil {
			e.logger.Warnf("cancel active orders error: %v", err)
		}
	}

	orderForm, err := e.generateOrder()
	if err != nil {
		return err
	} else if orderForm == nil {
		return nil
	}

	createdOrder, err := e.exchange.SubmitOrder(ctx, *orderForm)
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

func (e *FixedQuantityExecutor) generateOrder() (orderForm *types.SubmitOrder, err error) {
	newPrice, err := e.getNewPrice()
	if err != nil {
		return nil, err
	}

	minQuantity := e.market.MinQuantity
	base := e.position.GetBase()

	restQuantity := e.targetQuantity.Sub(base.Abs())

	if restQuantity.Sign() <= 0 {
		if e.cancelContextIfTargetQuantityFilled() {
			return nil, nil
		}
	}

	if restQuantity.Compare(minQuantity) < 0 {
		return nil, fmt.Errorf("can not continue placing orders, rest quantity %s is less than the min quantity %s", restQuantity.String(), minQuantity.String())
	}

	// when slice = 1000, if we only have 998, we should adjust our quantity to 998
	orderQuantity := fixedpoint.Min(e.sliceQuantity, restQuantity)

	// if the rest quantity in the next round is not enough, we should merge the rest quantity into this round
	// if there are rest slices
	nextRestQuantity := restQuantity.Sub(e.sliceQuantity)
	if nextRestQuantity.Sign() > 0 && nextRestQuantity.Compare(minQuantity) < 0 {
		orderQuantity = restQuantity
	}

	minNotional := e.market.MinNotional
	orderQuantity = bbgo.AdjustQuantityByMinAmount(orderQuantity, newPrice, minNotional)

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
			orderQuantity = bbgo.AdjustQuantityByMaxAmount(orderQuantity, newPrice, b.Available)
		}
	}

	if e.deadlineTime != nil && !e.deadlineTime.IsZero() {
		now := time.Now()
		if now.After(*e.deadlineTime) {
			orderForm = &types.SubmitOrder{
				Symbol:   e.symbol,
				Side:     e.side,
				Type:     types.OrderTypeMarket,
				Quantity: restQuantity,
				Market:   e.market,
			}
			return orderForm, nil
		}
	}

	orderForm = &types.SubmitOrder{
		Symbol:      e.symbol,
		Side:        e.side,
		Type:        types.OrderTypeLimitMaker,
		Quantity:    orderQuantity,
		Price:       newPrice,
		Market:      e.market,
		TimeInForce: "GTC",
	}

	return orderForm, err
}

func (e *FixedQuantityExecutor) Start(ctx context.Context) error {
	if e.marketDataStream != nil {
		return errors.New("market data stream is not nil, you can't start the executor twice")
	}

	e.executionCtx, e.cancelExecution = context.WithCancel(ctx)
	e.userDataStreamCtx, e.cancelUserDataStream = context.WithCancel(ctx)

	e.marketDataStream = e.exchange.NewStream()
	e.marketDataStream.SetPublicOnly()
	e.marketDataStream.Subscribe(types.BookChannel, e.symbol, types.SubscribeOptions{
		Depth: types.DepthLevelMedium,
	})

	e.orderBook = types.NewStreamBook(e.symbol)
	e.orderBook.BindStream(e.marketDataStream)

	// private channels
	e.userDataStream = e.exchange.NewStream()
	e.orderStore.BindStream(e.userDataStream)
	e.activeMakerOrders = bbgo.NewActiveOrderBook(e.symbol)
	e.activeMakerOrders.OnFilled(e.handleFilledOrder)
	e.activeMakerOrders.BindStream(e.userDataStream)
	e.tradeCollector.OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		e.logger.Info(trade.String())
	})
	e.tradeCollector.BindStream(e.userDataStream)

	go e.connectMarketData(e.executionCtx)
	go e.connectUserData(e.userDataStreamCtx)
	go e.orderUpdater(e.executionCtx)
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

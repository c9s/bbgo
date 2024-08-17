package twap

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// StreamExecutor is a TWAP execution that places orders on the best price by connecting to the real time market data stream.
type StreamExecutor struct {
	Session        *bbgo.ExchangeSession
	Symbol         string
	Side           types.SideType
	TargetQuantity fixedpoint.Value
	SliceQuantity  fixedpoint.Value
	StopPrice      fixedpoint.Value
	NumOfTicks     int
	UpdateInterval time.Duration
	DeadlineTime   time.Time

	market           types.Market
	marketDataStream types.Stream

	userDataStream       types.Stream
	userDataStreamCtx    context.Context
	cancelUserDataStream context.CancelFunc

	orderBook      *types.StreamOrderBook
	currentPrice   fixedpoint.Value
	activePosition fixedpoint.Value

	activeMakerOrders *bbgo.ActiveOrderBook
	orderStore        *core.OrderStore
	position          *types.Position

	executionCtx    context.Context
	cancelExecution context.CancelFunc

	stoppedC chan struct{}

	mu sync.Mutex
}

func (e *StreamExecutor) connectMarketData(ctx context.Context) {
	logrus.Infof("connecting market data stream...")
	if err := e.marketDataStream.Connect(ctx); err != nil {
		logrus.WithError(err).Errorf("market data stream connect error")
	}
}

func (e *StreamExecutor) connectUserData(ctx context.Context) {
	logrus.Infof("connecting user data stream...")
	if err := e.userDataStream.Connect(ctx); err != nil {
		logrus.WithError(err).Errorf("user data stream connect error")
	}
}

func (e *StreamExecutor) newBestPriceOrder() (orderForm types.SubmitOrder, err error) {
	book := e.orderBook.Copy()
	sideBook := book.SideBook(e.Side)

	first, ok := sideBook.First()
	if !ok {
		return orderForm, fmt.Errorf("empty %s %s side book", e.Symbol, e.Side)
	}

	newPrice := first.Price
	spread, ok := book.Spread()
	if !ok {
		return orderForm, errors.New("can not calculate spread, neither bid price or ask price exists")
	}

	// for example, we have tickSize = 0.01, and spread is 28.02 - 28.00 = 0.02
	// assign tickSpread = min(spread - tickSize, tickSpread)
	//
	// if number of ticks = 0, than the tickSpread is 0
	// tickSpread = min(0.02 - 0.01, 0)
	// price = first bid price 28.00 + tickSpread (0.00) = 28.00
	//
	// if number of ticks = 1, than the tickSpread is 0.01
	// tickSpread = min(0.02 - 0.01, 0.01)
	// price = first bid price 28.00 + tickSpread (0.01) = 28.01
	//
	// if number of ticks = 2, than the tickSpread is 0.02
	// tickSpread = min(0.02 - 0.01, 0.02)
	// price = first bid price 28.00 + tickSpread (0.01) = 28.01
	tickSize := e.market.TickSize
	tickSpread := tickSize.Mul(fixedpoint.NewFromInt(int64(e.NumOfTicks)))
	if spread.Compare(tickSize) > 0 {
		// there is a gap in the spread
		tickSpread = fixedpoint.Min(tickSpread, spread.Sub(tickSize))
		switch e.Side {
		case types.SideTypeSell:
			newPrice = newPrice.Sub(tickSpread)
		case types.SideTypeBuy:
			newPrice = newPrice.Add(tickSpread)
		}
	}

	if e.StopPrice.Sign() > 0 {
		switch e.Side {
		case types.SideTypeSell:
			if newPrice.Compare(e.StopPrice) < 0 {
				logrus.Infof("%s order price %s is lower than the stop sell price %s, setting order price to the stop sell price %s",
					e.Symbol,
					newPrice.String(),
					e.StopPrice.String(),
					e.StopPrice.String())
				newPrice = e.StopPrice
			}

		case types.SideTypeBuy:
			if newPrice.Compare(e.StopPrice) > 0 {
				logrus.Infof("%s order price %s is higher than the stop buy price %s, setting order price to the stop buy price %s",
					e.Symbol,
					newPrice.String(),
					e.StopPrice.String(),
					e.StopPrice.String())
				newPrice = e.StopPrice
			}
		}
	}

	minQuantity := e.market.MinQuantity
	base := e.position.GetBase()

	restQuantity := e.TargetQuantity.Sub(base.Abs())

	if restQuantity.Sign() <= 0 {
		if e.cancelContextIfTargetQuantityFilled() {
			return
		}
	}

	if restQuantity.Compare(minQuantity) < 0 {
		return orderForm, fmt.Errorf("can not continue placing orders, rest quantity %s is less than the min quantity %s", restQuantity.String(), minQuantity.String())
	}

	// when slice = 1000, if we only have 998, we should adjust our quantity to 998
	orderQuantity := fixedpoint.Min(e.SliceQuantity, restQuantity)

	// if the rest quantity in the next round is not enough, we should merge the rest quantity into this round
	// if there are rest slices
	nextRestQuantity := restQuantity.Sub(e.SliceQuantity)
	if nextRestQuantity.Sign() > 0 && nextRestQuantity.Compare(minQuantity) < 0 {
		orderQuantity = restQuantity
	}

	minNotional := e.market.MinNotional
	orderQuantity = bbgo.AdjustQuantityByMinAmount(orderQuantity, newPrice, minNotional)

	switch e.Side {
	case types.SideTypeSell:
		// check base balance for sell, try to sell as more as possible
		if b, ok := e.Session.GetAccount().Balance(e.market.BaseCurrency); ok {
			orderQuantity = fixedpoint.Min(b.Available, orderQuantity)
		}

	case types.SideTypeBuy:
		// check base balance for sell, try to sell as more as possible
		if b, ok := e.Session.GetAccount().Balance(e.market.QuoteCurrency); ok {
			orderQuantity = bbgo.AdjustQuantityByMaxAmount(orderQuantity, newPrice, b.Available)
		}
	}

	if !e.DeadlineTime.IsZero() {
		now := time.Now()
		if now.After(e.DeadlineTime) {
			orderForm = types.SubmitOrder{
				Symbol:   e.Symbol,
				Side:     e.Side,
				Type:     types.OrderTypeMarket,
				Quantity: restQuantity,
				Market:   e.market,
			}
			return orderForm, nil
		}
	}

	orderForm = types.SubmitOrder{
		// ClientOrderID:    "",
		Symbol:      e.Symbol,
		Side:        e.Side,
		Type:        types.OrderTypeLimitMaker,
		Quantity:    orderQuantity,
		Price:       newPrice,
		Market:      e.market,
		TimeInForce: "GTC",
	}
	return orderForm, err
}

func (e *StreamExecutor) updateOrder(ctx context.Context) error {
	book := e.orderBook.Copy()
	sideBook := book.SideBook(e.Side)

	first, ok := sideBook.First()
	if !ok {
		return fmt.Errorf("empty %s %s side book", e.Symbol, e.Side)
	}

	// if there is no gap between the first price entry and the second price entry
	second, ok := sideBook.Second()
	if !ok {
		return fmt.Errorf("no secoond price on the %s order book %s, can not update", e.Symbol, e.Side)
	}

	tickSize := e.market.TickSize
	numOfTicks := fixedpoint.NewFromInt(int64(e.NumOfTicks))
	tickSpread := tickSize.Mul(numOfTicks)

	// check and see if we need to cancel the existing active orders
	for e.activeMakerOrders.NumOfOrders() > 0 {
		orders := e.activeMakerOrders.Orders()

		if len(orders) > 1 {
			logrus.Warnf("more than 1 %s open orders in the strategy...", e.Symbol)
		}

		// get the first order
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

		switch e.Side {
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

		e.cancelActiveOrders()
	}

	orderForm, err := e.newBestPriceOrder()
	if err != nil {
		return err
	}

	createdOrders, err := e.Session.OrderExecutor.SubmitOrders(ctx, orderForm)
	if err != nil {
		return err
	}

	e.activeMakerOrders.Add(createdOrders...)
	e.orderStore.Add(createdOrders...)
	return nil
}

func (e *StreamExecutor) cancelActiveOrders() {
	gracefulCtx, gracefulCancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer gracefulCancel()
	e.activeMakerOrders.GracefulCancel(gracefulCtx, e.Session.Exchange)
}

func (e *StreamExecutor) orderUpdater(ctx context.Context) {
	updateLimiter := rate.NewLimiter(rate.Every(3*time.Second), 1)
	ticker := time.NewTimer(e.UpdateInterval)
	defer ticker.Stop()

	// we should stop updater and clean up our open orders, if
	// 1. the given context is canceled.
	// 2. the base quantity equals to or greater than the target quantity
	defer func() {
		e.cancelActiveOrders()
		e.cancelUserDataStream()
		e.emitDone()
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case <-e.orderBook.C:
			if !updateLimiter.Allow() {
				break
			}

			if e.cancelContextIfTargetQuantityFilled() {
				return
			}

			logrus.Infof("%s order book changed, checking order...", e.Symbol)
			if err := e.updateOrder(ctx); err != nil {
				logrus.WithError(err).Errorf("order update failed")
			}

		case <-ticker.C:
			if !updateLimiter.Allow() {
				break
			}

			if e.cancelContextIfTargetQuantityFilled() {
				return
			}

			if err := e.updateOrder(ctx); err != nil {
				logrus.WithError(err).Errorf("order update failed")
			}

		}
	}
}

func (e *StreamExecutor) cancelContextIfTargetQuantityFilled() bool {
	base := e.position.GetBase()

	if base.Abs().Compare(e.TargetQuantity) >= 0 {
		logrus.Infof("filled target quantity, canceling the order execution context")
		e.cancelExecution()
		return true
	}

	return false
}

func (e *StreamExecutor) handleTradeUpdate(trade types.Trade) {
	// ignore trades that are not in the symbol we interested
	if trade.Symbol != e.Symbol {
		return
	}

	if !e.orderStore.Exists(trade.OrderID) {
		return
	}

	logrus.Info(trade.String())

	e.position.AddTrade(trade)
	logrus.Infof("position updated: %+v", e.position)
}

func (e *StreamExecutor) handleFilledOrder(order types.Order) {
	logrus.Info(order.String())

	// filled event triggers the order removal from the active order store
	// we need to ensure we received every order update event before the execution is done.
	e.cancelContextIfTargetQuantityFilled()
}

func (e *StreamExecutor) Run(parentCtx context.Context) error {
	e.mu.Lock()
	e.stoppedC = make(chan struct{})
	e.executionCtx, e.cancelExecution = context.WithCancel(parentCtx)
	e.userDataStreamCtx, e.cancelUserDataStream = context.WithCancel(context.Background())
	e.mu.Unlock()

	if e.UpdateInterval == 0 {
		e.UpdateInterval = 10 * time.Second
	}

	var ok bool
	e.market, ok = e.Session.Market(e.Symbol)
	if !ok {
		return fmt.Errorf("market %s not found", e.Symbol)
	}

	e.marketDataStream = e.Session.Exchange.NewStream()
	e.marketDataStream.SetPublicOnly()
	e.marketDataStream.Subscribe(types.BookChannel, e.Symbol, types.SubscribeOptions{})

	e.orderBook = types.NewStreamBook(e.Symbol)
	e.orderBook.BindStream(e.marketDataStream)

	e.userDataStream = e.Session.Exchange.NewStream()
	e.userDataStream.OnTradeUpdate(e.handleTradeUpdate)

	e.position = &types.Position{
		Symbol:        e.Symbol,
		BaseCurrency:  e.market.BaseCurrency,
		QuoteCurrency: e.market.QuoteCurrency,
	}

	e.orderStore = core.NewOrderStore(e.Symbol)
	e.orderStore.BindStream(e.userDataStream)
	e.activeMakerOrders = bbgo.NewActiveOrderBook(e.Symbol)
	e.activeMakerOrders.OnFilled(e.handleFilledOrder)
	e.activeMakerOrders.BindStream(e.userDataStream)

	go e.connectMarketData(e.executionCtx)
	go e.connectUserData(e.userDataStreamCtx)
	go e.orderUpdater(e.executionCtx)
	return nil
}

func (e *StreamExecutor) emitDone() {
	e.mu.Lock()
	if e.stoppedC == nil {
		e.stoppedC = make(chan struct{})
	}
	close(e.stoppedC)
	e.mu.Unlock()
}

func (e *StreamExecutor) Done() (c <-chan struct{}) {
	e.mu.Lock()
	// if the channel is not allocated, it means it's not started yet, we need to return a closed channel
	if e.stoppedC == nil {
		e.stoppedC = make(chan struct{})
		close(e.stoppedC)
		c = e.stoppedC
	} else {
		c = e.stoppedC
	}

	e.mu.Unlock()
	return c
}

// Shutdown stops the execution
// If we call this method, it means the execution is still running,
// We need to:
// 1. stop the order updater (by using the execution context)
// 2. the order updater cancels all open orders and close the user data stream
func (e *StreamExecutor) Shutdown(shutdownCtx context.Context) {
	e.mu.Lock()
	if e.cancelExecution != nil {
		e.cancelExecution()
	}
	e.mu.Unlock()

	for {
		select {

		case <-shutdownCtx.Done():
			return

		case <-e.Done():
			return

		}
	}
}

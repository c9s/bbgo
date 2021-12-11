package bbgo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

type TwapExecution struct {
	Session        *ExchangeSession
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

	activeMakerOrders *LocalActiveOrderBook
	orderStore        *OrderStore
	position          *types.Position

	executionCtx    context.Context
	cancelExecution context.CancelFunc

	stoppedC chan struct{}

	state int

	mu sync.Mutex
}

func (e *TwapExecution) connectMarketData(ctx context.Context) {
	log.Infof("connecting market data stream...")
	if err := e.marketDataStream.Connect(ctx); err != nil {
		log.WithError(err).Errorf("market data stream connect error")
	}
}

func (e *TwapExecution) connectUserData(ctx context.Context) {
	log.Infof("connecting user data stream...")
	if err := e.userDataStream.Connect(ctx); err != nil {
		log.WithError(err).Errorf("user data stream connect error")
	}
}

func (e *TwapExecution) newBestPriceOrder() (orderForm types.SubmitOrder, err error) {
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
	tickSize := fixedpoint.NewFromFloat(e.market.TickSize)
	tickSpread := tickSize.MulInt(e.NumOfTicks)
	if spread > tickSize {
		// there is a gap in the spread
		tickSpread = fixedpoint.Min(tickSpread, spread-tickSize)
		switch e.Side {
		case types.SideTypeSell:
			newPrice -= tickSpread
		case types.SideTypeBuy:
			newPrice += tickSpread
		}
	}

	if e.StopPrice > 0 {
		switch e.Side {
		case types.SideTypeSell:
			if newPrice < e.StopPrice {
				log.Infof("%s order price %f is lower than the stop sell price %f, setting order price to the stop sell price %f",
					e.Symbol,
					newPrice.Float64(),
					e.StopPrice.Float64(),
					e.StopPrice.Float64())
				newPrice = e.StopPrice
			}

		case types.SideTypeBuy:
			if newPrice > e.StopPrice {
				log.Infof("%s order price %f is higher than the stop buy price %f, setting order price to the stop buy price %f",
					e.Symbol,
					newPrice.Float64(),
					e.StopPrice.Float64(),
					e.StopPrice.Float64())
				newPrice = e.StopPrice
			}
		}
	}

	minQuantity := fixedpoint.NewFromFloat(e.market.MinQuantity)

	e.position.Lock()
	base := e.position.Base
	e.position.Unlock()

	restQuantity := e.TargetQuantity - fixedpoint.Abs(base)

	if restQuantity <= 0 {
		if e.cancelContextIfTargetQuantityFilled() {
			return
		}
	}

	if restQuantity < minQuantity {
		return orderForm, fmt.Errorf("can not continue placing orders, rest quantity %f is less than the min quantity %f", restQuantity.Float64(), minQuantity.Float64())
	}

	// when slice = 1000, if we only have 998, we should adjust our quantity to 998
	orderQuantity := fixedpoint.Min(e.SliceQuantity, restQuantity)

	// if the rest quantity in the next round is not enough, we should merge the rest quantity into this round
	// if there are rest slices
	nextRestQuantity := restQuantity - e.SliceQuantity
	if nextRestQuantity > 0 && nextRestQuantity < minQuantity {
		orderQuantity = restQuantity
	}

	minNotional := fixedpoint.NewFromFloat(e.market.MinNotional)
	orderQuantity = AdjustQuantityByMinAmount(orderQuantity, newPrice, minNotional)

	switch e.Side {
	case types.SideTypeSell:
		// check base balance for sell, try to sell as more as possible
		if b, ok := e.Session.Account.Balance(e.market.BaseCurrency); ok {
			orderQuantity = fixedpoint.Min(b.Available, orderQuantity)
		}

	case types.SideTypeBuy:
		// check base balance for sell, try to sell as more as possible
		if b, ok := e.Session.Account.Balance(e.market.QuoteCurrency); ok {
			orderQuantity = AdjustQuantityByMaxAmount(orderQuantity, newPrice, b.Available)
		}
	}

	if e.DeadlineTime != emptyTime {
		now := time.Now()
		if now.After(e.DeadlineTime) {
			orderForm = types.SubmitOrder{
				Symbol:   e.Symbol,
				Side:     e.Side,
				Type:     types.OrderTypeMarket,
				Quantity: restQuantity.Float64(),
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
		Quantity:    orderQuantity.Float64(),
		Price:       newPrice.Float64(),
		Market:      e.market,
		TimeInForce: "GTC",
	}
	return orderForm, err
}

func (e *TwapExecution) updateOrder(ctx context.Context) error {
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

	tickSize := fixedpoint.NewFromFloat(e.market.TickSize)
	tickSpread := tickSize.MulInt(e.NumOfTicks)

	// check and see if we need to cancel the existing active orders
	for e.activeMakerOrders.NumOfOrders() > 0 {
		orders := e.activeMakerOrders.Orders()

		if len(orders) > 1 {
			log.Warnf("more than 1 %s open orders in the strategy...", e.Symbol)
		}

		// get the first order
		order := orders[0]
		orderPrice := fixedpoint.NewFromFloat(order.Price)
		// quantity := fixedpoint.NewFromFloat(order.Quantity)

		remainingQuantity := order.Quantity - order.ExecutedQuantity
		if remainingQuantity <= e.market.MinQuantity {
			log.Infof("order remaining quantity %f is less than the market minimal quantity %f, skip updating order", remainingQuantity, e.market.MinQuantity)
			return nil
		}

		// if the first bid price or first ask price is the same to the current active order
		// we should skip updating the order
		// DO NOT UPDATE IF:
		//   tickSpread > 0 AND current order price == second price + tickSpread
		//   current order price == first price
		log.Infof("orderPrice = %f first.Price = %f second.Price = %f tickSpread = %f", orderPrice.Float64(), first.Price.Float64(), second.Price.Float64(), tickSpread.Float64())

		switch e.Side {
		case types.SideTypeBuy:
			if tickSpread > 0 && orderPrice == second.Price+tickSpread {
				log.Infof("the current order is already on the best ask price %f", orderPrice.Float64())
				return nil
			} else if orderPrice == first.Price {
				log.Infof("the current order is already on the best bid price %f", orderPrice.Float64())
				return nil
			}

		case types.SideTypeSell:
			if tickSpread > 0 && orderPrice == second.Price-tickSpread {
				log.Infof("the current order is already on the best ask price %f", orderPrice.Float64())
				return nil
			} else if orderPrice == first.Price {
				log.Infof("the current order is already on the best ask price %f", orderPrice.Float64())
				return nil
			}
		}

		e.cancelActiveOrders(ctx)
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

func (e *TwapExecution) cancelActiveOrders(ctx context.Context) {
	didCancel := false
	for e.activeMakerOrders.NumOfOrders() > 0 {
		didCancel = true

		orders := e.activeMakerOrders.Orders()
		log.Infof("canceling %d open orders:", len(orders))
		e.activeMakerOrders.Print()

		if err := e.Session.Exchange.CancelOrders(ctx, orders...); err != nil {
			log.WithError(err).Errorf("can not cancel %s orders", e.Symbol)
		}

		select {
		case <-ctx.Done():
			return

		case <-time.After(3 * time.Second):

		}

		// verify the current open orders via the RESTful API
		if e.activeMakerOrders.NumOfOrders() > 0 {
			log.Warnf("there are orders not cancelled, using REStful API to verify...")
			openOrders, err := e.Session.Exchange.QueryOpenOrders(ctx, e.Symbol)
			if err != nil {
				log.WithError(err).Errorf("can not query %s open orders", e.Symbol)
				continue
			}

			openOrderStore := NewOrderStore(e.Symbol)
			openOrderStore.Add(openOrders...)

			for _, o := range e.activeMakerOrders.Orders() {
				// if it does not exist, we should remove it
				if !openOrderStore.Exists(o.OrderID) {
					e.activeMakerOrders.Remove(o)
				}
			}
		}
	}

	if didCancel {
		log.Infof("orders are canceled successfully")
	}
}

func (e *TwapExecution) orderUpdater(ctx context.Context) {
	updateLimiter := rate.NewLimiter(rate.Every(3*time.Second), 1)
	ticker := time.NewTimer(e.UpdateInterval)
	defer ticker.Stop()

	// we should stop updater and clean up our open orders, if
	// 1. the given context is canceled.
	// 2. the base quantity equals to or greater than the target quantity
	defer func() {
		e.cancelActiveOrders(context.Background())
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

			log.Infof("%s order book changed, checking order...", e.Symbol)
			if err := e.updateOrder(ctx); err != nil {
				log.WithError(err).Errorf("order update failed")
			}

		case <-ticker.C:
			if !updateLimiter.Allow() {
				break
			}

			if e.cancelContextIfTargetQuantityFilled() {
				return
			}

			if err := e.updateOrder(ctx); err != nil {
				log.WithError(err).Errorf("order update failed")
			}

		}
	}
}

func (e *TwapExecution) cancelContextIfTargetQuantityFilled() bool {
	e.position.Lock()
	base := e.position.Base
	e.position.Unlock()

	if fixedpoint.Abs(base) >= e.TargetQuantity {
		log.Infof("filled target quantity, canceling the order execution context")
		e.cancelExecution()
		return true
	}
	return false
}

func (e *TwapExecution) handleTradeUpdate(trade types.Trade) {
	// ignore trades that are not in the symbol we interested
	if trade.Symbol != e.Symbol {
		return
	}

	if !e.orderStore.Exists(trade.OrderID) {
		return
	}

	log.Info(trade.String())

	e.position.AddTrade(trade)
	log.Infof("position updated: %+v", e.position)
}

func (e *TwapExecution) handleFilledOrder(order types.Order) {
	log.Info(order.String())

	// filled event triggers the order removal from the active order store
	// we need to ensure we received every order update event before the execution is done.
	e.cancelContextIfTargetQuantityFilled()
}

func (e *TwapExecution) Run(parentCtx context.Context) error {
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
	go e.connectMarketData(e.executionCtx)

	e.userDataStream = e.Session.Exchange.NewStream()
	e.userDataStream.OnTradeUpdate(e.handleTradeUpdate)
	e.position = &types.Position{
		Symbol:        e.Symbol,
		BaseCurrency:  e.market.BaseCurrency,
		QuoteCurrency: e.market.QuoteCurrency,
	}

	e.orderStore = NewOrderStore(e.Symbol)
	e.orderStore.BindStream(e.userDataStream)
	e.activeMakerOrders = NewLocalActiveOrderBook()
	e.activeMakerOrders.OnFilled(e.handleFilledOrder)
	e.activeMakerOrders.BindStream(e.userDataStream)

	go e.connectUserData(e.userDataStreamCtx)
	go e.orderUpdater(e.executionCtx)
	return nil
}

func (e *TwapExecution) emitDone() {
	e.mu.Lock()
	if e.stoppedC == nil {
		e.stoppedC = make(chan struct{})
	}
	close(e.stoppedC)
	e.mu.Unlock()
}

func (e *TwapExecution) Done() (c <-chan struct{}) {
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
func (e *TwapExecution) Shutdown(shutdownCtx context.Context) {
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

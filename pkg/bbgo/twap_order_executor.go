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

const OrderExecutionReady = 1

type TwapExecution struct {
	Session        *ExchangeSession
	Symbol         string
	Side           types.SideType
	TargetQuantity fixedpoint.Value
	SliceQuantity  fixedpoint.Value

	market           types.Market
	marketDataStream types.Stream
	userDataStream   types.Stream
	orderBook        *types.StreamOrderBook
	currentPrice     fixedpoint.Value
	activePosition   fixedpoint.Value

	activeMakerOrders *LocalActiveOrderBook
	orderStore        *OrderStore
	position          *Position

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

func (e *TwapExecution) getSideBook() (pvs types.PriceVolumeSlice, err error) {
	book := e.orderBook.Get()

	switch e.Side {
	case types.SideTypeSell:
		pvs = book.Asks

	case types.SideTypeBuy:
		pvs = book.Bids

	default:
		err = fmt.Errorf("invalid side type: %+v", e.Side)
	}

	return pvs, err
}

func (e *TwapExecution) newBestPriceMakerOrder() (orderForm types.SubmitOrder, err error) {
	book := e.orderBook.Get()

	sideBook, err := e.getSideBook()
	if err != nil {
		return orderForm, err
	}

	first, ok := sideBook.First()
	if !ok {
		return orderForm, fmt.Errorf("empty %s %s side book", e.Symbol, e.Side)
	}

	newPrice := first.Price

	spread, ok := book.Spread()
	if !ok {
		return orderForm, errors.New("can not calculate spread, neither bid price or ask price exists")
	}

	tickSize := fixedpoint.NewFromFloat(e.market.TickSize)
	if spread > tickSize {
		switch e.Side {
		case types.SideTypeSell:
			newPrice -= fixedpoint.NewFromFloat(e.market.TickSize)
		case types.SideTypeBuy:
			newPrice += fixedpoint.NewFromFloat(e.market.TickSize)
		}
	}

	orderForm = types.SubmitOrder{
		// ClientOrderID:    "",
		Symbol:      e.Symbol,
		Side:        e.Side,
		Type:        types.OrderTypeLimitMaker,
		Quantity:    e.SliceQuantity.Float64(),
		Price:       newPrice.Float64(),
		Market:      e.market,
		TimeInForce: "GTC",
	}
	return orderForm, err
}

func (e *TwapExecution) updateOrder(ctx context.Context) error {
	book := e.orderBook.Get()
	bestBid, _ := book.BestBid()
	bestAsk, _ := book.BestAsk()
	log.Infof("best bid %f, best ask %f", bestBid.Price.Float64(), bestAsk.Price.Float64())

	sideBook, err := e.getSideBook()
	if err != nil {
		return err
	}

	first, ok := sideBook.First()
	if !ok {
		return fmt.Errorf("empty %s %s side book", e.Symbol, e.Side)
	}

	tickSize := fixedpoint.NewFromFloat(e.market.TickSize)

	// check and see if we need to cancel the existing active orders
	for e.activeMakerOrders.NumOfOrders() > 0 {
		orders := e.activeMakerOrders.Orders()

		if len(orders) > 1 {
			log.Warnf("there are more than 1 open orders in the strategy...")
		}

		// get the first order
		order := orders[0]
		price := fixedpoint.NewFromFloat(order.Price)
		quantity := fixedpoint.NewFromFloat(order.Quantity)

		// if the first bid price or first ask price is the same to the current active order
		// we should skip updating the order
		if first.Price == price {
			// there are other orders in the same price, it means if we cancel ours, the price is still the best price.
			if first.Volume > quantity {
				return nil
			}

			// if there is no gap between the first price entry and the second price entry
			second, ok := sideBook.Second()
			if !ok {
				return fmt.Errorf("there is no secoond price on the %s order book %s", e.Symbol, e.Side)
			}

			log.Infof("checking second price %f - %f")
			// if there is no gap
			if fixedpoint.Abs(first.Price-second.Price) == tickSize {
				return nil
			}
		}

		log.Infof("canceling orders...")
		if err := e.Session.Exchange.CancelOrders(ctx, orders...); err != nil {
			log.WithError(err).Errorf("can not cancel %s orders", e.Symbol)
		}

		time.Sleep(3 * time.Second)
	}

	orderForm, err := e.newBestPriceMakerOrder()
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

func (e *TwapExecution) orderUpdater(ctx context.Context) {
	rateLimiter := rate.NewLimiter(rate.Every(time.Minute), 15)
	ticker := time.NewTimer(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return

		case <-e.orderBook.C:
			if !rateLimiter.Allow() {
				break
			}

			if err := e.updateOrder(ctx); err != nil {
				log.WithError(err).Errorf("order update failed")
			}

		case <-ticker.C:
			if !rateLimiter.Allow() {
				break
			}

			if err := e.updateOrder(ctx); err != nil {
				log.WithError(err).Errorf("order update failed")
			}

		}
	}
}

func (e TwapExecution) tradeUpdateHandler(trade types.Trade) {
	// ignore trades that are not in the symbol we interested
	if trade.Symbol != e.Symbol {
		return
	}

	if !e.orderStore.Exists(trade.OrderID) {
		return
	}

	q := fixedpoint.NewFromFloat(trade.Quantity)
	_ = q

	e.position.AddTrade(trade)

	log.Infof("position updated: %+v", e.position)
}

func (e *TwapExecution) Run(ctx context.Context) error {
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
	go e.connectMarketData(ctx)

	e.userDataStream = e.Session.Exchange.NewStream()
	e.userDataStream.OnTradeUpdate(e.tradeUpdateHandler)
	e.position = &Position{
		Symbol:        e.Symbol,
		BaseCurrency:  e.market.BaseCurrency,
		QuoteCurrency: e.market.QuoteCurrency,
	}

	e.orderStore = NewOrderStore(e.Symbol)
	e.orderStore.BindStream(e.userDataStream)
	e.activeMakerOrders = NewLocalActiveOrderBook()
	e.activeMakerOrders.BindStream(e.userDataStream)

	go e.connectUserData(ctx)
	go e.orderUpdater(ctx)
	return nil
}

type TwapOrderExecutor struct {
	Session *ExchangeSession

	// Execution parameters
	// DelayTime is the order update delay time
	DelayTime types.Duration
}

func (e *TwapOrderExecutor) Execute(ctx context.Context, symbol string, side types.SideType, targetQuantity, sliceQuantity fixedpoint.Value) (*TwapExecution, error) {
	execution := &TwapExecution{
		Session:        e.Session,
		Symbol:         symbol,
		Side:           side,
		TargetQuantity: targetQuantity,
		SliceQuantity:  sliceQuantity,
	}
	err := execution.Run(ctx)
	return execution, err
}

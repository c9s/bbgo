package bbgo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var ErrExceededSubmitOrderRetryLimit = errors.New("exceeded submit order retry limit")

// quantityReduceDelta is used to modify the order to submit, especially for the market order
var quantityReduceDelta = fixedpoint.NewFromFloat(0.005)

// submitOrderRetryLimit is used when SubmitOrder failed, we will re-submit the order.
// This is for the maximum retries
const submitOrderRetryLimit = 5

// GeneralOrderExecutor implements the general order executor for strategy
type GeneralOrderExecutor struct {
	session            *ExchangeSession
	symbol             string
	strategy           string
	strategyInstanceID string
	position           *types.Position
	activeMakerOrders  *ActiveOrderBook
	orderStore         *OrderStore
	tradeCollector     *TradeCollector

	marginBaseMaxBorrowable, marginQuoteMaxBorrowable fixedpoint.Value

	closing int64
}

func NewGeneralOrderExecutor(session *ExchangeSession, symbol, strategy, strategyInstanceID string, position *types.Position) *GeneralOrderExecutor {
	// Always update the position fields
	position.Strategy = strategy
	position.StrategyInstanceID = strategyInstanceID

	orderStore := NewOrderStore(symbol)

	executor := &GeneralOrderExecutor{
		session:            session,
		symbol:             symbol,
		strategy:           strategy,
		strategyInstanceID: strategyInstanceID,
		position:           position,
		activeMakerOrders:  NewActiveOrderBook(symbol),
		orderStore:         orderStore,
		tradeCollector:     NewTradeCollector(symbol, position, orderStore),
	}

	if session.Margin {
		executor.startMarginAssetUpdater(context.Background())
	}

	return executor
}

func (e *GeneralOrderExecutor) startMarginAssetUpdater(ctx context.Context) {
	marginService, ok := e.session.Exchange.(types.MarginBorrowRepayService)
	if !ok {
		log.Warnf("session %s (%T) exchange does not support MarginBorrowRepayService", e.session.Name, e.session.Exchange)
		return
	}

	go e.marginAssetMaxBorrowableUpdater(ctx, 30*time.Minute, marginService, e.position.Market)
}

func (e *GeneralOrderExecutor) updateMarginAssetMaxBorrowable(ctx context.Context, marginService types.MarginBorrowRepayService, market types.Market) {
	maxBorrowable, err := marginService.QueryMarginAssetMaxBorrowable(ctx, market.BaseCurrency)
	if err != nil {
		log.WithError(err).Errorf("can not query margin base asset %s max borrowable", market.BaseCurrency)
	} else {
		log.Infof("updating margin base asset %s max borrowable amount: %f", market.BaseCurrency, maxBorrowable.Float64())
		e.marginBaseMaxBorrowable = maxBorrowable
	}

	maxBorrowable, err = marginService.QueryMarginAssetMaxBorrowable(ctx, market.QuoteCurrency)
	if err != nil {
		log.WithError(err).Errorf("can not query margin quote asset %s max borrowable", market.QuoteCurrency)
	} else {
		log.Infof("updating margin quote asset %s max borrowable amount: %f", market.QuoteCurrency, maxBorrowable.Float64())
		e.marginQuoteMaxBorrowable = maxBorrowable
	}
}

func (e *GeneralOrderExecutor) marginAssetMaxBorrowableUpdater(ctx context.Context, interval time.Duration, marginService types.MarginBorrowRepayService, market types.Market) {
	t := time.NewTicker(util.MillisecondsJitter(interval, 500))
	defer t.Stop()

	e.updateMarginAssetMaxBorrowable(ctx, marginService, market)
	for {
		select {
		case <-ctx.Done():
			return

		case <-t.C:
			e.updateMarginAssetMaxBorrowable(ctx, marginService, market)
		}
	}
}

func (e *GeneralOrderExecutor) ActiveMakerOrders() *ActiveOrderBook {
	return e.activeMakerOrders
}

func (e *GeneralOrderExecutor) BindEnvironment(environ *Environment) {
	e.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		environ.RecordPosition(e.position, trade, profit)
	})
}

func (e *GeneralOrderExecutor) BindTradeStats(tradeStats *types.TradeStats) {
	e.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		if profit == nil {
			return
		}

		tradeStats.Add(profit)
	})
}

func (e *GeneralOrderExecutor) BindProfitStats(profitStats *types.ProfitStats) {
	e.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		profitStats.AddTrade(trade)
		if profit == nil {
			return
		}

		profitStats.AddProfit(*profit)

		Notify(profit)
		Notify(profitStats)
	})
}

func (e *GeneralOrderExecutor) Bind() {
	e.activeMakerOrders.BindStream(e.session.UserDataStream)
	e.orderStore.BindStream(e.session.UserDataStream)

	// trade notify
	e.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		Notify(trade)
	})

	e.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", position)
		Notify(position)
	})

	e.tradeCollector.BindStream(e.session.UserDataStream)
}

// CancelOrders cancels the given order objects directly
func (e *GeneralOrderExecutor) CancelOrders(ctx context.Context, orders ...types.Order) error {
	err := e.session.Exchange.CancelOrders(ctx, orders...)
	if err != nil { // Retry once
		err = e.session.Exchange.CancelOrders(ctx, orders...)
	}
	return err
}

func (e *GeneralOrderExecutor) SubmitOrders(ctx context.Context, submitOrders ...types.SubmitOrder) (types.OrderSlice, error) {
	formattedOrders, err := e.session.FormatOrders(submitOrders)
	if err != nil {
		return nil, err
	}

	createdOrders, errIdx, err := BatchPlaceOrder(ctx, e.session.Exchange, formattedOrders...)
	if len(errIdx) > 0 {
		createdOrders2, err2 := BatchRetryPlaceOrder(ctx, e.session.Exchange, errIdx, formattedOrders...)
		if err2 != nil {
			err = multierr.Append(err, err2)
		} else {
			createdOrders = append(createdOrders, createdOrders2...)
		}
	}

	e.orderStore.Add(createdOrders...)
	e.activeMakerOrders.Add(createdOrders...)
	e.tradeCollector.Process()
	return createdOrders, err
}

type OpenPositionOptions struct {
	// Long is for open a long position
	// Long or Short must be set, avoid loading it from the config file
	// it should be set from the strategy code
	Long bool `json:"-" yaml:"-"`

	// Short is for open a short position
	// Long or Short must be set
	Short bool `json:"-" yaml:"-"`

	// Leverage is used for leveraged position and account
	// Leverage is not effected when using non-leverage spot account
	Leverage fixedpoint.Value `json:"leverage,omitempty" modifiable:"true"`

	// Quantity will be used first, it will override the leverage if it's given
	Quantity fixedpoint.Value `json:"quantity,omitempty" modifiable:"true"`

	// LimitOrder set to true to open a position with a limit order
	// default is false, and will send MarketOrder
	LimitOrder bool `json:"limitOrder,omitempty" modifiable:"true"`

	// LimitOrderTakerRatio is used when LimitOrder = true, it adjusts the price of the limit order with a ratio.
	// So you can ensure that the limit order can be a taker order. Higher the ratio, higher the chance it could be a taker order.
	//
	// limitOrderTakerRatio is the price ratio to adjust your limit order as a taker order. e.g., 0.1%
	// for sell order, 0.1% ratio means your final price = price * (1 - 0.1%)
	// for buy order, 0.1% ratio means your final price = price * (1 + 0.1%)
	// this is only enabled when the limitOrder option set to true
	LimitOrderTakerRatio fixedpoint.Value `json:"limitOrderTakerRatio,omitempty"`

	Price fixedpoint.Value `json:"-" yaml:"-"`
	Tags  []string         `json:"-" yaml:"-"`
}

func (e *GeneralOrderExecutor) reduceQuantityAndSubmitOrder(ctx context.Context, price fixedpoint.Value, submitOrder types.SubmitOrder) (types.OrderSlice, error) {
	var err error
	for i := 0; i < submitOrderRetryLimit; i++ {
		q := submitOrder.Quantity.Mul(fixedpoint.One.Sub(quantityReduceDelta))
		if !e.session.Futures {
			if submitOrder.Side == types.SideTypeSell {
				if baseBalance, ok := e.session.GetAccount().Balance(e.position.Market.BaseCurrency); ok {
					q = fixedpoint.Min(q, baseBalance.Available)
				}
			} else {
				if quoteBalance, ok := e.session.GetAccount().Balance(e.position.Market.QuoteCurrency); ok {
					q = fixedpoint.Min(q, quoteBalance.Available.Div(price))
				}
			}
		}
		log.Warnf("retrying order, adjusting order quantity: %v -> %v", submitOrder.Quantity, q)

		submitOrder.Quantity = q
		if e.position.Market.IsDustQuantity(submitOrder.Quantity, price) {
			return nil, types.NewZeroAssetError(nil)
		}

		createdOrder, err2 := e.SubmitOrders(ctx, submitOrder)
		if err2 != nil {
			// collect the error object
			err = multierr.Append(err, err2)
			continue
		}

		log.Infof("created order: %+v", createdOrder)
		return createdOrder, nil
	}

	return nil, multierr.Append(ErrExceededSubmitOrderRetryLimit, err)
}

func (e *GeneralOrderExecutor) OpenPosition(ctx context.Context, options OpenPositionOptions) (types.OrderSlice, error) {
	price := options.Price
	submitOrder := types.SubmitOrder{
		Symbol:           e.position.Symbol,
		Type:             types.OrderTypeMarket,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
		Tag:              strings.Join(options.Tags, ","),
	}

	baseBalance, _ := e.session.GetAccount().Balance(e.position.Market.BaseCurrency)

	// FIXME: fix the max quote borrowing checking
	// quoteBalance, _ := e.session.Account.Balance(e.position.Market.QuoteCurrency)

	if !options.LimitOrderTakerRatio.IsZero() {
		if options.Price.IsZero() {
			return nil, fmt.Errorf("OpenPositionOptions.Price is zero, can not adjust limit taker order price, options given: %+v", options)
		}

		if options.Long {
			// use higher price to buy (this ensures that our order will be filled)
			price = price.Mul(one.Add(options.LimitOrderTakerRatio))
		} else if options.Short {
			// use lower price to sell (this ensures that our order will be filled)
			price = price.Mul(one.Sub(options.LimitOrderTakerRatio))
		}
	}

	if options.LimitOrder {
		submitOrder.Type = types.OrderTypeLimit
		submitOrder.Price = price
	}

	quantity := options.Quantity

	if options.Long {
		if quantity.IsZero() {
			quoteQuantity, err := CalculateQuoteQuantity(ctx, e.session, e.position.QuoteCurrency, options.Leverage)
			if err != nil {
				return nil, err
			}

			quantity = quoteQuantity.Div(price)
		}
		if e.position.Market.IsDustQuantity(quantity, price) {
			log.Warnf("dust quantity: %v", quantity)
			return nil, nil
		}

		quoteQuantity := quantity.Mul(price)
		if e.session.Margin && !e.marginQuoteMaxBorrowable.IsZero() && quoteQuantity.Compare(e.marginQuoteMaxBorrowable) > 0 {
			log.Warnf("adjusting quantity %f according to the max margin quote borrowable amount: %f", quantity.Float64(), e.marginQuoteMaxBorrowable.Float64())
			quantity = AdjustQuantityByMaxAmount(quantity, price, e.marginQuoteMaxBorrowable)
		}

		submitOrder.Side = types.SideTypeBuy
		submitOrder.Quantity = quantity

		Notify("Opening %s long position with quantity %v at price %v", e.position.Symbol, quantity, price)

		createdOrder, err := e.SubmitOrders(ctx, submitOrder)
		if err == nil {
			return createdOrder, nil
		}

		return e.reduceQuantityAndSubmitOrder(ctx, price, submitOrder)
	} else if options.Short {
		if quantity.IsZero() {
			var err error
			quantity, err = CalculateBaseQuantity(e.session, e.position.Market, price, quantity, options.Leverage)
			if err != nil {
				return nil, err
			}
		}
		if e.position.Market.IsDustQuantity(quantity, price) {
			log.Warnf("dust quantity: %v", quantity)
			return nil, nil
		}

		if e.session.Margin && !e.marginBaseMaxBorrowable.IsZero() && quantity.Sub(baseBalance.Available).Compare(e.marginBaseMaxBorrowable) > 0 {
			log.Warnf("adjusting %f quantity according to the max margin base borrowable amount: %f", quantity.Float64(), e.marginBaseMaxBorrowable.Float64())
			// quantity = fixedpoint.Min(quantity, e.marginBaseMaxBorrowable)
			quantity = baseBalance.Available.Add(e.marginBaseMaxBorrowable)
		}

		submitOrder.Side = types.SideTypeSell
		submitOrder.Quantity = quantity

		Notify("Opening %s short position with quantity %v at price %v", e.position.Symbol, quantity, price)
		return e.reduceQuantityAndSubmitOrder(ctx, price, submitOrder)
	}

	return nil, errors.New("options Long or Short must be set")
}

// GracefulCancelActiveOrderBook cancels the orders from the active orderbook.
func (e *GeneralOrderExecutor) GracefulCancelActiveOrderBook(ctx context.Context, activeOrders *ActiveOrderBook, orders ...types.Order) error {
	if activeOrders.NumOfOrders() == 0 {
		return nil
	}
	if err := activeOrders.GracefulCancel(ctx, e.session.Exchange, orders...); err != nil {
		// Retry once
		if err = activeOrders.GracefulCancel(ctx, e.session.Exchange); err != nil {
			return fmt.Errorf("graceful cancel order error: %w", err)
		}
	}

	e.tradeCollector.Process()
	return nil
}

// CancelActiveOrderBookNoWait cancels the orders from the active orderbook without waiting
func (e *GeneralOrderExecutor) CancelActiveOrderBookNoWait(ctx context.Context, activeOrders *ActiveOrderBook, orders ...types.Order) error {
	if activeOrders.NumOfOrders() == 0 {
		return nil
	}
	if err := activeOrders.CancelNoWait(ctx, e.session.Exchange, orders...); err != nil {
		return fmt.Errorf("cancel order error: %w", err)
	}
	return nil
}

// GracefulCancel cancels all active maker orders if orders are not given, otherwise cancel all the given orders
func (e *GeneralOrderExecutor) GracefulCancel(ctx context.Context, orders ...types.Order) error {
	return e.GracefulCancelActiveOrderBook(ctx, e.activeMakerOrders, orders...)
}

// CancelNoWait cancels all active maker orders if orders is not given, otherwise cancel the given orders
func (e *GeneralOrderExecutor) CancelNoWait(ctx context.Context, orders ...types.Order) error {
	return e.CancelActiveOrderBookNoWait(ctx, e.activeMakerOrders, orders...)
}

// ClosePosition closes the current position by a percentage.
// percentage 0.1 means close 10% position
// tag is the order tag you want to attach, you may pass multiple tags, the tags will be combined into one tag string by commas.
func (e *GeneralOrderExecutor) ClosePosition(ctx context.Context, percentage fixedpoint.Value, tags ...string) error {
	submitOrder := e.position.NewMarketCloseOrder(percentage)
	if submitOrder == nil {
		return nil
	}

	if e.closing > 0 {
		log.Errorf("position is already closing")
		return nil
	}

	atomic.AddInt64(&e.closing, 1)
	defer atomic.StoreInt64(&e.closing, 0)

	if e.session.Futures { // Futures: Use base qty in e.position
		submitOrder.Quantity = e.position.GetBase().Abs()
		submitOrder.ReduceOnly = true
		if e.position.IsLong() {
			submitOrder.Side = types.SideTypeSell
		} else if e.position.IsShort() {
			submitOrder.Side = types.SideTypeBuy
		} else {
			submitOrder.Side = types.SideTypeSelf
			submitOrder.Quantity = fixedpoint.Zero
		}

		if submitOrder.Quantity.IsZero() {
			return fmt.Errorf("no position to close: %+v", submitOrder)
		}
	} else { // Spot and spot margin
		// check base balance and adjust the close position order
		if e.position.IsLong() {
			if baseBalance, ok := e.session.Account.Balance(e.position.Market.BaseCurrency); ok {
				submitOrder.Quantity = fixedpoint.Min(submitOrder.Quantity, baseBalance.Available)
			}
			if submitOrder.Quantity.IsZero() {
				return fmt.Errorf("insufficient base balance, can not sell: %+v", submitOrder)
			}
		} else if e.position.IsShort() {
			// TODO: check quote balance here, we also need the current price to validate, need to design.
			/*
				if quoteBalance, ok := e.session.Account.Balance(e.position.Market.QuoteCurrency); ok {
					// AdjustQuantityByMaxAmount(submitOrder.Quantity, quoteBalance.Available)
					// submitOrder.Quantity = fixedpoint.Min(submitOrder.Quantity,)
				}
			*/
		}
	}

	tagStr := strings.Join(tags, ",")
	submitOrder.Tag = tagStr

	Notify("Closing %s position %s with tags: %v", e.symbol, percentage.Percentage(), tagStr)

	_, err := e.SubmitOrders(ctx, *submitOrder)
	return err
}

func (e *GeneralOrderExecutor) TradeCollector() *TradeCollector {
	return e.tradeCollector
}

func (e *GeneralOrderExecutor) Session() *ExchangeSession {
	return e.session
}

func (e *GeneralOrderExecutor) Position() *types.Position {
	return e.position
}

// This implements PositionReader interface
func (e *GeneralOrderExecutor) CurrentPosition() *types.Position {
	return e.position
}

// This implements PositionResetter interface
func (e *GeneralOrderExecutor) ResetPosition() error {
	e.position.Reset()
	return nil
}

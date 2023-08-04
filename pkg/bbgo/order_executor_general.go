package bbgo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/backoff"
)

var ErrExceededSubmitOrderRetryLimit = errors.New("exceeded submit order retry limit")

// quantityReduceDelta is used to modify the order to submit, especially for the market order
var quantityReduceDelta = fixedpoint.NewFromFloat(0.005)

// submitOrderRetryLimit is used when SubmitOrder failed, we will re-submit the order.
// This is for the maximum retries
const submitOrderRetryLimit = 5

type BaseOrderExecutor struct {
	session           *ExchangeSession
	activeMakerOrders *ActiveOrderBook
	orderStore        *core.OrderStore
}

func (e *BaseOrderExecutor) OrderStore() *core.OrderStore {
	return e.orderStore
}

func (e *BaseOrderExecutor) ActiveMakerOrders() *ActiveOrderBook {
	return e.activeMakerOrders
}

// GracefulCancel cancels all active maker orders if orders are not given, otherwise cancel all the given orders
func (e *BaseOrderExecutor) GracefulCancel(ctx context.Context, orders ...types.Order) error {
	if err := e.activeMakerOrders.GracefulCancel(ctx, e.session.Exchange, orders...); err != nil {
		return errors.Wrap(err, "graceful cancel error")
	}

	return nil
}

// GeneralOrderExecutor implements the general order executor for strategy
type GeneralOrderExecutor struct {
	BaseOrderExecutor

	symbol             string
	strategy           string
	strategyInstanceID string
	position           *types.Position
	tradeCollector     *core.TradeCollector

	logger log.FieldLogger

	marginBaseMaxBorrowable, marginQuoteMaxBorrowable fixedpoint.Value

	maxRetries    uint
	disableNotify bool
}

func NewGeneralOrderExecutor(session *ExchangeSession, symbol, strategy, strategyInstanceID string, position *types.Position) *GeneralOrderExecutor {
	// Always update the position fields
	position.Strategy = strategy
	position.StrategyInstanceID = strategyInstanceID

	orderStore := core.NewOrderStore(symbol)

	executor := &GeneralOrderExecutor{
		BaseOrderExecutor: BaseOrderExecutor{
			session:           session,
			activeMakerOrders: NewActiveOrderBook(symbol),
			orderStore:        orderStore,
		},

		symbol:             symbol,
		strategy:           strategy,
		strategyInstanceID: strategyInstanceID,
		position:           position,
		tradeCollector:     core.NewTradeCollector(symbol, position, orderStore),
	}

	if session != nil && session.Margin {
		executor.startMarginAssetUpdater(context.Background())
	}

	return executor
}

func (e *GeneralOrderExecutor) DisableNotify() {
	e.disableNotify = true
}

func (e *GeneralOrderExecutor) SetMaxRetries(maxRetries uint) {
	e.maxRetries = maxRetries
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

		if !e.disableNotify {
			Notify(profit)
			Notify(profitStats)
		}
	})
}

func (e *GeneralOrderExecutor) Bind() {
	e.activeMakerOrders.BindStream(e.session.UserDataStream)
	e.orderStore.BindStream(e.session.UserDataStream)

	if !e.disableNotify {
		// trade notify
		e.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
			Notify(trade)
		})

		e.tradeCollector.OnPositionUpdate(func(position *types.Position) {
			log.Infof("position changed: %s", position)
			Notify(position)
		})
	}

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

func (e *GeneralOrderExecutor) SetLogger(logger log.FieldLogger) {
	e.logger = logger
}

func (e *GeneralOrderExecutor) SubmitOrders(ctx context.Context, submitOrders ...types.SubmitOrder) (types.OrderSlice, error) {
	formattedOrders, err := e.session.FormatOrders(submitOrders)
	if err != nil {
		return nil, err
	}

	orderCreateCallback := func(createdOrder types.Order) {
		e.orderStore.Add(createdOrder)
		e.activeMakerOrders.Add(createdOrder)
	}

	defer e.tradeCollector.Process()

	if e.maxRetries == 0 {
		createdOrders, _, err := BatchPlaceOrder(ctx, e.session.Exchange, orderCreateCallback, formattedOrders...)
		return createdOrders, err
	}

	createdOrders, _, err := BatchRetryPlaceOrder(ctx, e.session.Exchange, nil, orderCreateCallback, e.logger, formattedOrders...)
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
		if !e.session.Futures && !e.session.Margin {
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
			return nil, types.NewZeroAssetError(fmt.Errorf("dust quantity, quantity = %f, price = %f", submitOrder.Quantity.Float64(), price.Float64()))
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

// Create new submitOrder from OpenPositionOptions.
// @param ctx: golang context type.
// @param options: OpenPositionOptions to control the generated SubmitOrder in a higher level way. Notice that the Price in options will be updated as the submitOrder price.
// @return *types.SubmitOrder: SubmitOrder with calculated quantity and price.
// @return error: Error message.
func (e *GeneralOrderExecutor) NewOrderFromOpenPosition(ctx context.Context, options *OpenPositionOptions) (*types.SubmitOrder, error) {
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
			options.Price = price
		} else if options.Short {
			// use lower price to sell (this ensures that our order will be filled)
			price = price.Mul(one.Sub(options.LimitOrderTakerRatio))
			options.Price = price
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

			if price.IsZero() {
				return nil, errors.New("unable to calculate quantity: zero price given")
			}

			quantity = quoteQuantity.Div(price)
		}

		if e.position.Market.IsDustQuantity(quantity, price) {
			log.Errorf("can not submit order: dust quantity, quantity = %f, price = %f", quantity.Float64(), price.Float64())
			return nil, nil
		}

		quoteQuantity := quantity.Mul(price)
		if e.session.Margin && !e.marginQuoteMaxBorrowable.IsZero() && quoteQuantity.Compare(e.marginQuoteMaxBorrowable) > 0 {
			log.Warnf("adjusting quantity %f according to the max margin quote borrowable amount: %f", quantity.Float64(), e.marginQuoteMaxBorrowable.Float64())
			quantity = AdjustQuantityByMaxAmount(quantity, price, e.marginQuoteMaxBorrowable)
		}

		submitOrder.Side = types.SideTypeBuy
		submitOrder.Quantity = quantity

		return &submitOrder, nil
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

		return &submitOrder, nil
	}

	return nil, errors.New("options Long or Short must be set")
}

// OpenPosition sends the orders generated from OpenPositionOptions to the exchange by calling SubmitOrders or reduceQuantityAndSubmitOrder.
// @param ctx: golang context type.
// @param options: OpenPositionOptions to control the generated SubmitOrder in a higher level way. Notice that the Price in options will be updated as the submitOrder price.
// @return types.OrderSlice: Created orders with information from exchange.
// @return error: Error message.
func (e *GeneralOrderExecutor) OpenPosition(ctx context.Context, options OpenPositionOptions) (types.OrderSlice, error) {
	if e.position.IsClosing() {
		return nil, errors.Wrap(ErrPositionAlreadyClosing, "unable to open position")
	}

	submitOrder, err := e.NewOrderFromOpenPosition(ctx, &options)
	if err != nil {
		return nil, err
	}

	if submitOrder == nil {
		return nil, nil
	}

	price := options.Price

	side := "long"
	if submitOrder.Side == types.SideTypeSell {
		side = "short"
	}

	Notify("Opening %s %s position with quantity %f at price %f", e.position.Symbol, side, submitOrder.Quantity.Float64(), price.Float64())

	createdOrder, err := e.SubmitOrders(ctx, *submitOrder)
	if err == nil {
		return createdOrder, nil
	}

	log.WithError(err).Errorf("unable to submit order: %v", err)
	log.Infof("reduce quantity and retry order")
	return e.reduceQuantityAndSubmitOrder(ctx, price, *submitOrder)
}

// GracefulCancelActiveOrderBook cancels the orders from the active orderbook.
func (e *GeneralOrderExecutor) GracefulCancelActiveOrderBook(ctx context.Context, activeOrders *ActiveOrderBook) error {
	if activeOrders.NumOfOrders() == 0 {
		return nil
	}

	defer e.tradeCollector.Process()

	op := func() error { return activeOrders.GracefulCancel(ctx, e.session.Exchange) }
	return backoff.RetryGeneral(ctx, op)
}

// GracefulCancel cancels all active maker orders if orders are not given, otherwise cancel all the given orders
func (e *GeneralOrderExecutor) GracefulCancel(ctx context.Context, orders ...types.Order) error {
	if err := e.activeMakerOrders.GracefulCancel(ctx, e.session.Exchange, orders...); err != nil {
		return errors.Wrap(err, "graceful cancel error")
	}

	return nil
}

var ErrPositionAlreadyClosing = errors.New("position is already in closing process")

// ClosePosition closes the current position by a percentage.
// percentage 0.1 means close 10% position
// tag is the order tag you want to attach, you may pass multiple tags, the tags will be combined into one tag string by commas.
func (e *GeneralOrderExecutor) ClosePosition(ctx context.Context, percentage fixedpoint.Value, tags ...string) error {
	if !e.position.SetClosing(true) {
		return ErrPositionAlreadyClosing
	}
	defer e.position.SetClosing(false)

	submitOrder := e.position.NewMarketCloseOrder(percentage)
	if submitOrder == nil {
		return nil
	}

	if e.session.Futures { // Futures: Use base qty in e.position
		submitOrder.Quantity = e.position.GetBase().Abs()
		submitOrder.ReduceOnly = true

		if e.position.IsLong() {
			submitOrder.Side = types.SideTypeSell
		} else if e.position.IsShort() {
			submitOrder.Side = types.SideTypeBuy
		} else {
			return fmt.Errorf("unexpected position side: %+v", e.position)
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

	Notify("Closing %s position %s with tags: %s", e.symbol, percentage.Percentage(), tagStr)

	createdOrders, err := e.SubmitOrders(ctx, *submitOrder)
	if err != nil {
		return err
	}

	if queryOrderService, ok := e.session.Exchange.(types.ExchangeOrderQueryService); ok && !IsBackTesting {
		switch submitOrder.Type {
		case types.OrderTypeMarket:
			_, err2 := retry.QueryOrderUntilFilled(ctx, queryOrderService, createdOrders[0].Symbol, createdOrders[0].OrderID)
			if err2 != nil {
				log.WithError(err2).Errorf("unable to query order")
			}
		}
	}

	return nil
}

func (e *GeneralOrderExecutor) TradeCollector() *core.TradeCollector {
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

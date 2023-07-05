package bbgo

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var DefaultSubmitOrderRetryTimeout = 5 * time.Minute

func init() {
	if du, ok := util.GetEnvVarDuration("BBGO_SUBMIT_ORDER_RETRY_TIMEOUT"); ok && du > 0 {
		DefaultSubmitOrderRetryTimeout = du
	}
}

type OrderExecutor interface {
	SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error)
	CancelOrders(ctx context.Context, orders ...types.Order) error
}

//go:generate mockgen -destination=mocks/mock_order_executor_extended.go -package=mocks . OrderExecutorExtended
type OrderExecutorExtended interface {
	SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error)
	CancelOrders(ctx context.Context, orders ...types.Order) error
	TradeCollector() *core.TradeCollector
	Position() *types.Position
}

type OrderExecutionRouter interface {
	// SubmitOrdersTo submit order to a specific exchange Session
	SubmitOrdersTo(ctx context.Context, session string, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error)
	CancelOrdersTo(ctx context.Context, session string, orders ...types.Order) error
}

type ExchangeOrderExecutionRouter struct {
	sessions  map[string]*ExchangeSession
	executors map[string]OrderExecutor
}

func (e *ExchangeOrderExecutionRouter) SubmitOrdersTo(ctx context.Context, session string, orders ...types.SubmitOrder) (types.OrderSlice, error) {
	if executor, ok := e.executors[session]; ok {
		return executor.SubmitOrders(ctx, orders...)
	}

	es, ok := e.sessions[session]
	if !ok {
		return nil, fmt.Errorf("exchange session %s not found", session)
	}

	formattedOrders, err := es.FormatOrders(orders)
	if err != nil {
		return nil, err
	}

	createdOrders, _, err := BatchPlaceOrder(ctx, es.Exchange, nil, formattedOrders...)
	return createdOrders, err
}

func (e *ExchangeOrderExecutionRouter) CancelOrdersTo(ctx context.Context, session string, orders ...types.Order) error {
	if executor, ok := e.executors[session]; ok {
		return executor.CancelOrders(ctx, orders...)
	}
	es, ok := e.sessions[session]
	if !ok {
		return fmt.Errorf("exchange session %s not found", session)
	}

	return es.Exchange.CancelOrders(ctx, orders...)
}

// ExchangeOrderExecutor is an order executor wrapper for single exchange instance.
//
//go:generate callbackgen -type ExchangeOrderExecutor
type ExchangeOrderExecutor struct {
	// MinQuoteBalance fixedpoint.Value `json:"minQuoteBalance,omitempty" yaml:"minQuoteBalance,omitempty"`

	Session *ExchangeSession `json:"-" yaml:"-"`

	// private trade update callbacks
	tradeUpdateCallbacks []func(trade types.Trade)

	// private order update callbacks
	orderUpdateCallbacks []func(order types.Order)
}

func (e *ExchangeOrderExecutor) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (types.OrderSlice, error) {
	formattedOrders, err := e.Session.FormatOrders(orders)
	if err != nil {
		return nil, err
	}

	for _, order := range formattedOrders {
		log.Infof("submitting order: %s", order.String())
	}

	createdOrders, _, err := BatchPlaceOrder(ctx, e.Session.Exchange, nil, formattedOrders...)
	return createdOrders, err
}

func (e *ExchangeOrderExecutor) CancelOrders(ctx context.Context, orders ...types.Order) error {
	for _, order := range orders {
		log.Infof("cancelling order: %s", order)
	}
	return e.Session.Exchange.CancelOrders(ctx, orders...)
}

type BasicRiskController struct {
	Logger *log.Logger

	MaxOrderAmount      fixedpoint.Value `json:"maxOrderAmount,omitempty" yaml:"maxOrderAmount,omitempty"`
	MinQuoteBalance     fixedpoint.Value `json:"minQuoteBalance,omitempty" yaml:"minQuoteBalance,omitempty"`
	MaxBaseAssetBalance fixedpoint.Value `json:"maxBaseAssetBalance,omitempty" yaml:"maxBaseAssetBalance,omitempty"`
	MinBaseAssetBalance fixedpoint.Value `json:"minBaseAssetBalance,omitempty" yaml:"minBaseAssetBalance,omitempty"`
}

// ProcessOrders filters and modifies the submit order objects by:
// 1. Increase the quantity by the minimal requirement
// 2. Decrease the quantity by risk controls
// 3. If the quantity does not meet minimal requirement, we should ignore the submit order.
func (c *BasicRiskController) ProcessOrders(session *ExchangeSession, orders ...types.SubmitOrder) (outOrders []types.SubmitOrder, errs []error) {
	balances := session.GetAccount().Balances()

	addError := func(err error) {
		errs = append(errs, err)
	}

	accumulativeQuoteAmount := fixedpoint.Zero
	accumulativeBaseSellQuantity := fixedpoint.Zero
	increaseFactor := fixedpoint.NewFromFloat(1.01)

	for _, order := range orders {
		lastPrice, ok := session.LastPrice(order.Symbol)
		if !ok {
			addError(fmt.Errorf("the last price of symbol %q is not found, order: %s", order.Symbol, order.String()))
			continue
		}

		market, ok := session.Market(order.Symbol)
		if !ok {
			addError(fmt.Errorf("the market config of symbol %q is not found, order: %s", order.Symbol, order.String()))
			continue
		}

		price := order.Price
		quantity := order.Quantity
		switch order.Type {
		case types.OrderTypeMarket:
			price = lastPrice
		}

		switch order.Side {
		case types.SideTypeBuy:
			minAmount := market.MinAmount.Mul(increaseFactor)
			// Critical conditions for placing buy orders
			quoteBalance, ok := balances[market.QuoteCurrency]
			if !ok {
				addError(fmt.Errorf("can not place buy order, quote balance %s not found", market.QuoteCurrency))
				continue
			}

			if quoteBalance.Available.Compare(c.MinQuoteBalance) < 0 {
				addError(errors.Wrapf(ErrQuoteBalanceLevelTooLow, "can not place buy order, quote balance level is too low: %s < %s, order: %s",
					types.USD.FormatMoney(quoteBalance.Available),
					types.USD.FormatMoney(c.MinQuoteBalance), order.String()))
				continue
			}

			// Increase the quantity if the amount is not enough,
			// this is the only increase op, later we will decrease the quantity if it meets the criteria
			quantity = AdjustFloatQuantityByMinAmount(quantity, price, minAmount)

			if c.MaxOrderAmount.Sign() > 0 {
				quantity = AdjustFloatQuantityByMaxAmount(quantity, price, c.MaxOrderAmount)
			}

			quoteAssetQuota := fixedpoint.Max(
				fixedpoint.Zero, quoteBalance.Available.Sub(c.MinQuoteBalance))
			if quoteAssetQuota.Compare(market.MinAmount) < 0 {
				addError(
					errors.Wrapf(
						ErrInsufficientQuoteBalance,
						"can not place buy order, insufficient quote balance: quota %s < min amount %s, order: %s",
						quoteAssetQuota.String(), market.MinAmount.String(), order.String()))
				continue
			}

			quantity = AdjustFloatQuantityByMaxAmount(quantity, price, quoteAssetQuota)

			// if MaxBaseAssetBalance is enabled, we should check the current base asset balance
			if baseBalance, hasBaseAsset := balances[market.BaseCurrency]; hasBaseAsset && c.MaxBaseAssetBalance.Sign() > 0 {
				if baseBalance.Available.Compare(c.MaxBaseAssetBalance) > 0 {
					addError(
						errors.Wrapf(
							ErrAssetBalanceLevelTooHigh,
							"should not place buy order, asset balance level is too high: %s > %s, order: %s",
							baseBalance.Available.String(),
							c.MaxBaseAssetBalance.String(),
							order.String()))
					continue
				}

				baseAssetQuota := fixedpoint.Max(fixedpoint.Zero, c.MaxBaseAssetBalance.Sub(baseBalance.Available))
				if quantity.Compare(baseAssetQuota) > 0 {
					quantity = baseAssetQuota
				}
			}

			// if the amount is still too small, we should skip it.
			notional := quantity.Mul(lastPrice)
			if notional.Compare(market.MinAmount) < 0 {
				addError(
					fmt.Errorf(
						"can not place buy order, quote amount too small: notional %s < min amount %s, order: %s",
						notional.String(),
						market.MinAmount.String(),
						order.String()))
				continue
			}

			accumulativeQuoteAmount = accumulativeQuoteAmount.Add(notional)

		case types.SideTypeSell:
			minNotion := market.MinNotional.Mul(increaseFactor)

			// Critical conditions for placing SELL orders
			baseAssetBalance, ok := balances[market.BaseCurrency]
			if !ok {
				addError(
					fmt.Errorf(
						"can not place sell order, no base asset balance %s, order: %s",
						market.BaseCurrency,
						order.String()))
				continue
			}

			// if the amount is too small, we should increase it.
			quantity = AdjustFloatQuantityByMinAmount(quantity, price, minNotion)

			// we should not SELL too much
			quantity = fixedpoint.Min(quantity, baseAssetBalance.Available)

			if c.MinBaseAssetBalance.Sign() > 0 {
				if baseAssetBalance.Available.Compare(c.MinBaseAssetBalance) < 0 {
					addError(
						errors.Wrapf(
							ErrAssetBalanceLevelTooLow,
							"asset balance level is too low: %s > %s", baseAssetBalance.Available.String(), c.MinBaseAssetBalance.String()))
					continue
				}

				quantity = fixedpoint.Min(quantity, baseAssetBalance.Available.Sub(c.MinBaseAssetBalance))
				if quantity.Compare(market.MinQuantity) < 0 {
					addError(
						errors.Wrapf(
							ErrInsufficientAssetBalance,
							"insufficient asset balance: %s > minimal quantity %s",
							baseAssetBalance.Available.String(),
							market.MinQuantity.String()))
					continue
				}
			}

			if c.MaxOrderAmount.Sign() > 0 {
				quantity = AdjustFloatQuantityByMaxAmount(quantity, price, c.MaxOrderAmount)
			}

			notional := quantity.Mul(lastPrice)
			if notional.Compare(market.MinNotional) < 0 {
				addError(
					fmt.Errorf(
						"can not place sell order, notional %s < min notional: %s, order: %s",
						notional.String(),
						market.MinNotional.String(),
						order.String()))
				continue
			}

			if quantity.Compare(market.MinQuantity) < 0 {
				addError(
					fmt.Errorf(
						"can not place sell order, quantity %s is less than the minimal lot %s, order: %s",
						quantity.String(),
						market.MinQuantity.String(),
						order.String()))
				continue
			}

			accumulativeBaseSellQuantity = accumulativeBaseSellQuantity.Add(quantity)
		}

		// update quantity and format the order
		order.Quantity = quantity
		outOrders = append(outOrders, order)
	}

	return outOrders, nil
}

type OrderCallback func(order types.Order)

// BatchPlaceOrder
func BatchPlaceOrder(ctx context.Context, exchange types.Exchange, orderCallback OrderCallback, submitOrders ...types.SubmitOrder) (types.OrderSlice, []int, error) {
	var createdOrders types.OrderSlice
	var err error

	var errIndexes []int
	for i, submitOrder := range submitOrders {
		createdOrder, err2 := exchange.SubmitOrder(ctx, submitOrder)
		if err2 != nil {
			err = multierr.Append(err, err2)
			errIndexes = append(errIndexes, i)
		} else if createdOrder != nil {
			createdOrder.Tag = submitOrder.Tag

			if orderCallback != nil {
				orderCallback(*createdOrder)
			}

			createdOrders = append(createdOrders, *createdOrder)
		}
	}

	return createdOrders, errIndexes, err
}

// BatchRetryPlaceOrder places the orders and retries the failed orders
func BatchRetryPlaceOrder(ctx context.Context, exchange types.Exchange, errIdx []int, orderCallback OrderCallback, logger log.FieldLogger, submitOrders ...types.SubmitOrder) (types.OrderSlice, []int, error) {
	if logger == nil {
		logger = log.StandardLogger()
	}

	var createdOrders types.OrderSlice
	var werr error

	// if the errIdx is nil, then we should iterate all the submit orders
	// allocate a variable for new error index
	if len(errIdx) == 0 {
		var err2 error
		createdOrders, errIdx, err2 = BatchPlaceOrder(ctx, exchange, orderCallback, submitOrders...)
		if err2 != nil {
			werr = multierr.Append(werr, err2)
		} else {
			return createdOrders, nil, nil
		}
	}

	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, DefaultSubmitOrderRetryTimeout)
	defer cancelTimeout()

	// if we got any error, we should re-iterate the errored orders
	coolDownTime := 200 * time.Millisecond

	// set backoff max retries to 101 because https://ja.wikipedia.org/wiki/101%E5%9B%9E%E7%9B%AE%E3%81%AE%E3%83%97%E3%83%AD%E3%83%9D%E3%83%BC%E3%82%BA
	backoffMaxRetries := uint64(101)
	var errIdxNext []int
batchRetryOrder:
	for retryRound := 0; len(errIdx) > 0 && retryRound < 10; retryRound++ {
		// sleep for 200 millisecond between each retry
		logger.Warnf("retry round #%d, cooling down for %s", retryRound+1, coolDownTime)
		time.Sleep(coolDownTime)

		// reset error index since it's a new retry
		errIdxNext = nil

		// iterate the error index and re-submit the order
		logger.Warnf("starting retry round #%d...", retryRound+1)
		for _, idx := range errIdx {
			submitOrder := submitOrders[idx]

			op := func() error {
				// can allocate permanent error backoff.Permanent(err) to stop backoff
				createdOrder, err2 := exchange.SubmitOrder(timeoutCtx, submitOrder)
				if err2 != nil {
					logger.WithError(err2).Errorf("submit order error: %s", submitOrder.String())
				}

				if err2 == nil && createdOrder != nil {
					// if the order is successfully created, then we should copy the order tag
					createdOrder.Tag = submitOrder.Tag

					if orderCallback != nil {
						orderCallback(*createdOrder)
					}

					createdOrders = append(createdOrders, *createdOrder)
				}

				return err2
			}

			var bo backoff.BackOff = backoff.NewExponentialBackOff()
			bo = backoff.WithMaxRetries(bo, backoffMaxRetries)
			bo = backoff.WithContext(bo, timeoutCtx)
			if err2 := backoff.Retry(op, bo); err2 != nil {
				if err2 == context.Canceled {
					logger.Warnf("context canceled error, stop retry")
					break batchRetryOrder
				}

				werr = multierr.Append(werr, err2)
				errIdxNext = append(errIdxNext, idx)
			}
		}

		// update the error index
		errIdx = errIdxNext
	}

	return createdOrders, errIdx, werr
}

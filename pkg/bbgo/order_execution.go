package bbgo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type OrderExecutor interface {
	SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error)
	CancelOrders(ctx context.Context, orders ...types.Order) error

	OnTradeUpdate(cb func(trade types.Trade))
	OnOrderUpdate(cb func(order types.Order))
	EmitTradeUpdate(trade types.Trade)
	EmitOrderUpdate(order types.Order)
}

type OrderExecutionRouter interface {
	// SubmitOrdersTo submit order to a specific exchange Session
	SubmitOrdersTo(ctx context.Context, session string, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error)
	CancelOrdersTo(ctx context.Context, session string, orders ...types.Order) error
}

type ExchangeOrderExecutionRouter struct {
	Notifiability

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

	formattedOrders, err := formatOrders(es, orders)
	if err != nil {
		return nil, err
	}

	return es.Exchange.SubmitOrders(ctx, formattedOrders...)
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
//go:generate callbackgen -type ExchangeOrderExecutor
type ExchangeOrderExecutor struct {
	// MinQuoteBalance fixedpoint.Value `json:"minQuoteBalance,omitempty" yaml:"minQuoteBalance,omitempty"`

	Notifiability `json:"-" yaml:"-"`

	Session *ExchangeSession `json:"-" yaml:"-"`

	// private trade update callbacks
	tradeUpdateCallbacks []func(trade types.Trade)

	// private order update callbacks
	orderUpdateCallbacks []func(order types.Order)
}

func (e *ExchangeOrderExecutor) notifySubmitOrders(orders ...types.SubmitOrder) {
	for _, order := range orders {
		// pass submit order as an interface object.
		channel, ok := e.RouteObject(&order)
		if ok {
			e.NotifyTo(channel, ":memo: Submitting %s %s %s order with quantity: %f @ %f", order.Symbol, order.Type, order.Side, order.Quantity, order.Price, &order)
		} else {
			e.Notify(":memo: Submitting %s %s %s order with quantity: %f @ %f", order.Symbol, order.Type, order.Side, order.Quantity, order.Price, &order)
		}
	}
}

func (e *ExchangeOrderExecutor) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (types.OrderSlice, error) {
	formattedOrders, err := formatOrders(e.Session, orders)
	if err != nil {
		return nil, err
	}

	for _, order := range formattedOrders {
		// pass submit order as an interface object.
		channel, ok := e.RouteObject(&order)
		if ok {
			e.NotifyTo(channel, ":memo: Submitting %s %s %s order with quantity: %f", order.Symbol, order.Type, order.Side, order.Quantity, order)
		} else {
			e.Notify(":memo: Submitting %s %s %s order with quantity: %f", order.Symbol, order.Type, order.Side, order.Quantity, order)
		}

		log.Infof("submitting order: %s", order.String())
	}

	e.notifySubmitOrders(formattedOrders...)

	return e.Session.Exchange.SubmitOrders(ctx, formattedOrders...)
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
	balances := session.Account.Balances()

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

func formatOrders(session *ExchangeSession, orders []types.SubmitOrder) (formattedOrders []types.SubmitOrder, err error) {
	for _, order := range orders {
		o, err := session.FormatOrder(order)
		if err != nil {
			return formattedOrders, err
		}
		formattedOrders = append(formattedOrders, o)
	}

	return formattedOrders, err
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

package bbgo

import (
	"context"
	"fmt"
	"math"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ExchangeOrderExecutionRouter struct {
	Notifiability

	sessions map[string]*ExchangeSession
}

func (e *ExchangeOrderExecutionRouter) SubmitOrdersTo(ctx context.Context, session string, orders ...types.SubmitOrder) (types.OrderSlice, error) {
	es, ok := e.sessions[session]
	if !ok {
		return nil, fmt.Errorf("exchange Session %s not found", session)
	}

	formattedOrders, err := formatOrders(es, orders)
	if err != nil {
		return nil, err
	}

	return es.Exchange.SubmitOrders(ctx, formattedOrders...)
}

// ExchangeOrderExecutor is an order executor wrapper for single exchange instance.
type ExchangeOrderExecutor struct {
	Notifiability `json:"-"`

	Session *ExchangeSession
}

func (e *ExchangeOrderExecutor) notifySubmitOrders(orders ...types.SubmitOrder) {
	for _, order := range orders {
		// pass submit order as an interface object.
		channel, ok := e.RouteObject(&order)
		if ok {
			e.NotifyTo(channel, ":memo: Submitting %s %s %s order with quantity: %s at price: %s", order.Symbol, order.Type, order.Side, order.QuantityString, order.PriceString, &order)
		} else {
			e.Notify(":memo: Submitting %s %s %s order with quantity: %s at price: %s", order.Symbol, order.Type, order.Side, order.QuantityString, order.PriceString, &order)
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
			e.NotifyTo(channel, ":memo: Submitting %s %s %s order with quantity: %s", order.Symbol, order.Type, order.Side, order.QuantityString, order)
		} else {
			e.Notify(":memo: Submitting %s %s %s order with quantity: %s", order.Symbol, order.Type, order.Side, order.QuantityString, order)
		}
	}

	e.notifySubmitOrders(formattedOrders...)

	return e.Session.Exchange.SubmitOrders(ctx, formattedOrders...)
}

type BasicRiskController struct {
	Logger *logrus.Logger

	MaxOrderAmount      fixedpoint.Value `json:"maxOrderAmount,omitempty"`
	MinQuoteBalance     fixedpoint.Value `json:"minQuoteBalance,omitempty"`
	MaxBaseAssetBalance fixedpoint.Value `json:"maxBaseAssetBalance,omitempty"`
	MinBaseAssetBalance fixedpoint.Value `json:"minBaseAssetBalance,omitempty"`
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

	accumulativeQuoteAmount := 0.0
	accumulativeBaseSellQuantity := 0.0
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
			// Critical conditions for placing buy orders
			quoteBalance, ok := balances[market.QuoteCurrency]
			if !ok {
				addError(fmt.Errorf("can not place buy order, quote balance %s not found", market.QuoteCurrency))
				continue
			}

			if quoteBalance.Available < c.MinQuoteBalance {
				addError(errors.Wrapf(ErrQuoteBalanceLevelTooLow, "can not place buy order, quote balance level is too low: %s < %s, order: %s",
					types.USD.FormatMoneyFloat64(quoteBalance.Available.Float64()),
					types.USD.FormatMoneyFloat64(c.MinQuoteBalance.Float64()), order.String()))
				continue
			}

			// Increase the quantity if the amount is not enough,
			// this is the only increase op, later we will decrease the quantity if it meets the criteria
			quantity = adjustQuantityByMinAmount(quantity, price, market.MinAmount*1.01)

			if c.MaxOrderAmount > 0 {
				quantity = adjustQuantityByMaxAmount(quantity, price, c.MaxOrderAmount.Float64())
			}

			quoteAssetQuota := math.Max(0.0, quoteBalance.Available.Float64()-c.MinQuoteBalance.Float64())
			if quoteAssetQuota < market.MinAmount {
				addError(
					errors.Wrapf(
						ErrInsufficientQuoteBalance,
						"can not place buy order, insufficient quote balance: quota %f < min amount %f, order: %s",
						quoteAssetQuota, market.MinAmount, order.String()))
				continue
			}

			quantity = adjustQuantityByMaxAmount(quantity, price, quoteAssetQuota)

			// if MaxBaseAssetBalance is enabled, we should check the current base asset balance
			if baseBalance, hasBaseAsset := balances[market.BaseCurrency]; hasBaseAsset && c.MaxBaseAssetBalance > 0 {
				if baseBalance.Available > c.MaxBaseAssetBalance {
					addError(
						errors.Wrapf(
							ErrAssetBalanceLevelTooHigh,
							"should not place buy order, asset balance level is too high: %f > %f, order: %s",
							baseBalance.Available.Float64(),
							c.MaxBaseAssetBalance.Float64(),
							order.String()))
					continue
				}

				baseAssetQuota := math.Max(0.0, c.MaxBaseAssetBalance.Float64()-baseBalance.Available.Float64())
				if quantity > baseAssetQuota {
					quantity = baseAssetQuota
				}
			}

			// if the amount is still too small, we should skip it.
			notional := quantity * lastPrice
			if notional < market.MinAmount {
				addError(
					fmt.Errorf(
						"can not place buy order, quote amount too small: notional %f < min amount %f, order: %s",
						notional,
						market.MinAmount,
						order.String()))
				continue
			}

			accumulativeQuoteAmount += notional

		case types.SideTypeSell:
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
			quantity = adjustQuantityByMinAmount(quantity, price, market.MinNotional*1.01)

			// we should not SELL too much
			quantity = math.Min(quantity, baseAssetBalance.Available.Float64())

			if c.MinBaseAssetBalance > 0 {
				if baseAssetBalance.Available < c.MinBaseAssetBalance {
					addError(
						errors.Wrapf(
							ErrAssetBalanceLevelTooLow,
							"asset balance level is too low: %f > %f", baseAssetBalance.Available.Float64(), c.MinBaseAssetBalance.Float64()))
					continue
				}

				quantity = math.Min(quantity, baseAssetBalance.Available.Float64()-c.MinBaseAssetBalance.Float64())
				if quantity < market.MinQuantity {
					addError(
						errors.Wrapf(
							ErrInsufficientAssetBalance,
							"insufficient asset balance: %f > minimal quantity %f",
							baseAssetBalance.Available.Float64(),
							market.MinQuantity))
					continue
				}
			}

			if c.MaxOrderAmount > 0 {
				quantity = adjustQuantityByMaxAmount(quantity, price, c.MaxOrderAmount.Float64())
			}

			notional := quantity * lastPrice
			if notional < market.MinNotional {
				addError(
					fmt.Errorf(
						"can not place sell order, notional %f < min notional: %f, order: %s",
						notional,
						market.MinNotional,
						order.String()))
				continue
			}

			if quantity < market.MinLot {
				addError(
					fmt.Errorf(
						"can not place sell order, quantity %f is less than the minimal lot %f, order: %s",
						quantity,
						market.MinLot,
						order.String()))
				continue
			}

			accumulativeBaseSellQuantity += quantity
		}

		// update quantity and format the order
		order.Quantity = quantity
		outOrders = append(outOrders, order)
	}

	return outOrders, nil
}

func formatOrder(session *ExchangeSession, order types.SubmitOrder) (types.SubmitOrder, error) {
	market, ok := session.Market(order.Symbol)
	if !ok {
		return order, fmt.Errorf("market is not defined: %s", order.Symbol)
	}

	order.Market = market

	switch order.Type {
	case types.OrderTypeMarket, types.OrderTypeStopMarket:
		order.Price = 0.0
		order.PriceString = ""

	default:
		order.PriceString = market.FormatPrice(order.Price)

	}

	order.QuantityString = market.FormatQuantity(order.Quantity)
	return order, nil
}

func formatOrders(session *ExchangeSession, orders []types.SubmitOrder) (formattedOrders []types.SubmitOrder, err error) {
	for _, order := range orders {
		o, err := formatOrder(session, order)
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

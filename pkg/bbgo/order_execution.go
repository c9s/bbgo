package bbgo

import (
	"context"
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
		return nil, errors.Errorf("exchange session %s not found", session)
	}

	formattedOrders, err := formatOrders(orders, es)
	if err != nil {
		return nil, err
	}

	return es.Exchange.SubmitOrders(ctx, formattedOrders...)
}

// ExchangeOrderExecutor is an order executor wrapper for single exchange instance.
type ExchangeOrderExecutor struct {
	Notifiability `json:"-"`

	session *ExchangeSession `json:"-"`
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
	formattedOrders, err := formatOrders(orders, e.session)
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

	return e.session.Exchange.SubmitOrders(ctx, formattedOrders...)
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
func (c *BasicRiskController) ProcessOrders(session *ExchangeSession, orders ...types.SubmitOrder) (outOrders []types.SubmitOrder, err error) {
	balances := session.Account.Balances()

	accumulativeQuoteAmount := 0.0
	accumulativeBaseSellQuantity := 0.0
	for _, order := range orders {
		lastPrice, ok := session.LastPrice(order.Symbol)
		if !ok {
			c.Logger.Errorf("the last price of symbol %q is not found", order.Symbol)
			continue
		}

		market, ok := session.Market(order.Symbol)
		if !ok {
			c.Logger.Errorf("the market config of symbol %q is not found", order.Symbol)
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
				c.Logger.Errorf("can not place buy order, quote balance %s not found", market.QuoteCurrency)
				continue
			}

			if quoteBalance.Available < c.MinQuoteBalance.Float64() {
				c.Logger.WithError(ErrQuoteBalanceLevelTooLow).Errorf("can not place buy order, quote balance level is too low: %s < %s",
					types.USD.FormatMoneyFloat64(quoteBalance.Available),
					types.USD.FormatMoneyFloat64(c.MinQuoteBalance.Float64()))
				continue
			}

			// Increase the quantity if the amount is not enough,
			// this is the only increase op, later we will decrease the quantity if it meets the criteria
			quantity = adjustQuantityByMinAmount(quantity, price, market.MinAmount*1.01)

			if c.MaxOrderAmount > 0 {
				quantity = adjustQuantityByMaxAmount(quantity, price, c.MaxOrderAmount.Float64())
			}

			quoteAssetQuota := math.Max(0.0, quoteBalance.Available-c.MinQuoteBalance.Float64())
			if quoteAssetQuota < market.MinAmount {
				c.Logger.WithError(ErrInsufficientQuoteBalance).Errorf("can not place buy order, insufficient quote balance: quota %f < min amount %f", quoteAssetQuota, market.MinAmount)
				continue
			}

			quantity = adjustQuantityByMaxAmount(quantity, price, quoteAssetQuota)

			// if MaxBaseAssetBalance is enabled, we should check the current base asset balance
			if baseBalance, hasBaseAsset := balances[market.BaseCurrency]; hasBaseAsset && c.MaxBaseAssetBalance > 0 {
				if baseBalance.Available > c.MaxBaseAssetBalance.Float64() {
					c.Logger.WithError(ErrAssetBalanceLevelTooHigh).Errorf("should not place buy order, asset balance level is too high: %f > %f", baseBalance.Available, c.MaxBaseAssetBalance.Float64())
					continue
				}

				baseAssetQuota := math.Max(0, c.MaxBaseAssetBalance.Float64()-baseBalance.Available)
				if quantity > baseAssetQuota {
					quantity = baseAssetQuota
				}
			}

			// if the amount is still too small, we should skip it.
			notional := quantity * lastPrice
			if notional < market.MinAmount {
				c.Logger.Errorf("can not place buy order, quote amount too small: notional %f < min amount %f", notional, market.MinAmount)
				continue
			}

			accumulativeQuoteAmount += notional

		case types.SideTypeSell:
			// Critical conditions for placing SELL orders
			baseAssetBalance, ok := balances[market.BaseCurrency]
			if !ok {
				c.Logger.Errorf("can not place sell order, no base asset balance %s", market.BaseCurrency)
				continue
			}

			// if the amount is too small, we should increase it.
			quantity = adjustQuantityByMinAmount(quantity, price, market.MinNotional*1.01)

			// we should not SELL too much
			quantity = math.Min(quantity, baseAssetBalance.Available)

			if c.MinBaseAssetBalance > 0 {
				if baseAssetBalance.Available < c.MinBaseAssetBalance.Float64() {
					c.Logger.WithError(ErrAssetBalanceLevelTooLow).Errorf("asset balance level is too low: %f > %f", baseAssetBalance.Available, c.MinBaseAssetBalance.Float64())
					continue
				}

				quantity = math.Min(quantity, baseAssetBalance.Available-c.MinBaseAssetBalance.Float64())
				if quantity < market.MinQuantity {
					c.Logger.WithError(ErrInsufficientAssetBalance).Errorf("insufficient asset balance: %f > minimal quantity %f", baseAssetBalance.Available, market.MinQuantity)
					continue
				}
			}

			if c.MaxOrderAmount > 0 {
				quantity = adjustQuantityByMaxAmount(quantity, price, c.MaxOrderAmount.Float64())
			}

			notional := quantity * lastPrice
			if notional < market.MinNotional {
				c.Logger.Errorf("can not place sell order, notional %f < min notional: %f", notional, market.MinNotional)
				continue
			}

			if quantity < market.MinLot {
				c.Logger.Errorf("can not place sell order, quantity %f is less than the minimal lot %f", quantity, market.MinLot)
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

type BasicRiskControlOrderExecutor struct {
	*ExchangeOrderExecutor

	BasicRiskController
}

func (e *BasicRiskControlOrderExecutor) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (types.OrderSlice, error) {
	orders, _ = e.BasicRiskController.ProcessOrders(e.session, orders...)
	formattedOrders, _ := formatOrders(orders, e.session)

	e.notifySubmitOrders(formattedOrders...)

	return e.session.Exchange.SubmitOrders(ctx, formattedOrders...)
}

func formatOrder(order types.SubmitOrder, session *ExchangeSession) (types.SubmitOrder, error) {
	market, ok := session.Market(order.Symbol)
	if !ok {
		return order, errors.Errorf("market is not defined: %s", order.Symbol)
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

func formatOrders(orders []types.SubmitOrder, session *ExchangeSession) (formattedOrders []types.SubmitOrder, err error) {
	for _, order := range orders {
		o, err := formatOrder(order, session)
		if err != nil {
			return formattedOrders, err
		}
		formattedOrders = append(formattedOrders, o)
	}

	return formattedOrders, err
}

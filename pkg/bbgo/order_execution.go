package bbgo

import (
	"context"
	"fmt"
	"math"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ExchangeOrderExecutionRouter struct {
	Notifiability

	sessions map[string]*ExchangeSession
}

func (e *ExchangeOrderExecutionRouter) SubmitOrdersTo(ctx context.Context, session string, orders ...types.SubmitOrder) ([]types.Order, error) {
	es, ok := e.sessions[session]
	if !ok {
		return nil, errors.Errorf("exchange session %s not found", session)
	}

	formattedOrders, err := formatOrders(orders, es)
	if err != nil {
		return nil, err
	}

	// e.Notify(":memo: Submitting order to %s %s %s %s with quantity: %s", session, order.Symbol, order.Type, order.Side, order.QuantityString, order)
	return es.Exchange.SubmitOrders(ctx, formattedOrders...)
}

// ExchangeOrderExecutor is an order executor wrapper for single exchange instance.
type ExchangeOrderExecutor struct {
	Notifiability `json:"-"`

	session *ExchangeSession `json:"-"`
}

func (e *ExchangeOrderExecutor) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) ([]types.Order, error) {
	formattedOrders, err := formatOrders(orders, e.session)
	if err != nil {
		return nil, err
	}

	// e.Notify(":memo: Submitting %s %s %s order with quantity: %s", order.Symbol, order.Type, order.Side, order.QuantityString, order)

	return e.session.Exchange.SubmitOrders(ctx, formattedOrders...)
}

type RiskControlOrderExecutor struct {
	Notifiability `json:"-"`

	MinQuoteBalance fixedpoint.Value `json:"minQuoteBalance,omitempty"`
	MaxAssetBalance fixedpoint.Value `json:"maxBaseAssetBalance,omitempty"`
	MinAssetBalance fixedpoint.Value `json:"minBaseAssetBalance,omitempty"`
	MaxOrderAmount  fixedpoint.Value `json:"maxOrderAmount,omitempty"`

	session *ExchangeSession
}

func (e *RiskControlOrderExecutor) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) ([]types.Order, error) {
	var formattedOrders []types.SubmitOrder
	for _, order := range orders {
		currentPrice, ok := e.session.lastPrices[order.Symbol]
		if ok {
			return nil, errors.Errorf("the last price of symbol %q is not found", order.Symbol)
		}

		market := order.Market
		quantity := order.Quantity

		balances := e.session.Account.Balances()

		switch order.Side {
		case types.SideTypeBuy:

			if balance, ok := balances[market.QuoteCurrency]; ok {
				if balance.Available < e.MinQuoteBalance.Float64() {
					return nil, errors.Wrapf(ErrQuoteBalanceLevelTooLow, "quote balance level is too low: %s < %s",
						types.USD.FormatMoneyFloat64(balance.Available),
						types.USD.FormatMoneyFloat64(e.MinQuoteBalance.Float64()))
				}

				if baseBalance, ok := balances[market.BaseCurrency]; ok {
					if e.MaxAssetBalance > 0 && baseBalance.Available > e.MaxAssetBalance.Float64() {
						return nil, errors.Wrapf(ErrAssetBalanceLevelTooHigh, "asset balance level is too high: %f > %f", baseBalance.Available, e.MaxAssetBalance)
					}
				}

				available := math.Max(0.0, balance.Available-e.MinQuoteBalance.Float64())
				if available < market.MinAmount {
					return nil, errors.Wrapf(ErrInsufficientQuoteBalance, "insufficient quote balance: %f < min amount %f", available, market.MinAmount)
				}

				quantity = adjustQuantityByMinAmount(quantity, currentPrice, market.MinAmount*1.01)
				quantity = adjustQuantityByMaxAmount(quantity, currentPrice, available)
				amount := quantity * currentPrice
				if amount < market.MinAmount {
					return nil, fmt.Errorf("amount too small: %f < min amount %f", amount, market.MinAmount)
				}
			}

		case types.SideTypeSell:

			if balance, ok := balances[market.BaseCurrency]; ok {
				if e.MinAssetBalance > 0 && balance.Available < e.MinAssetBalance.Float64() {
					return nil, errors.Wrapf(ErrAssetBalanceLevelTooLow, "asset balance level is too low: %f > %f", balance.Available, e.MinAssetBalance)
				}

				quantity = adjustQuantityByMinAmount(quantity, currentPrice, market.MinNotional*1.01)

				available := balance.Available
				quantity = math.Min(quantity, available)
				if quantity < market.MinQuantity {
					return nil, errors.Wrapf(ErrInsufficientAssetBalance, "insufficient asset balance: %f > minimal quantity %f", available, market.MinQuantity)
				}

				notional := quantity * currentPrice
				if notional < market.MinNotional {
					return nil, fmt.Errorf("notional %f < min notional: %f", notional, market.MinNotional)
				}

				if quantity < market.MinLot {
					return nil, fmt.Errorf("quantity %f less than min lot %f", quantity, market.MinLot)
				}

				notional = quantity * currentPrice
				if notional < market.MinNotional {
					return nil, fmt.Errorf("notional %f < min notional: %f", notional, market.MinNotional)
				}
			}
		}

		// udpate quantity and format the order
		order.Quantity = quantity
		o, err := formatOrder(order, e.session)
		if err != nil {
			return nil, err
		}

		formattedOrders = append(formattedOrders, o)
	}

	// e.Notify(":memo: Submitting %s %s %s order with quantity: %s", order.Symbol, order.Type, order.Side, order.QuantityString, order)
	return e.session.Exchange.SubmitOrders(ctx, formattedOrders...)
}

func formatOrder(order types.SubmitOrder, session *ExchangeSession) (types.SubmitOrder, error) {
	market, ok := session.Market(order.Symbol)
	if !ok {
		return order, errors.Errorf("market is not defined: %s", order.Symbol)
	}

	order.Market = market
	order.PriceString = market.FormatPrice(order.Price)
	order.QuantityString = market.FormatVolume(order.Quantity)
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

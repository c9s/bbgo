package bbgo

import (
	"context"

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

	session *ExchangeSession `json:"-"`
}

func (e *RiskControlOrderExecutor) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) ([]types.Order, error) {
	formattedOrders, err := formatOrders(orders, e.session)
	if err != nil {
		return nil, err
	}

	// e.Notify(":memo: Submitting %s %s %s order with quantity: %s", order.Symbol, order.Type, order.Side, order.QuantityString, order)
	return e.session.Exchange.SubmitOrders(ctx, formattedOrders...)
}

func formatOrders(orders []types.SubmitOrder, session *ExchangeSession) (formattedOrders []types.SubmitOrder, err error) {
	for _, order := range orders {
		market, ok := session.Market(order.Symbol)
		if !ok {
			return formattedOrders, errors.Errorf("market is not defined: %s", order.Symbol)
		}

		order.Market = market
		order.PriceString = market.FormatPrice(order.Price)
		order.QuantityString = market.FormatVolume(order.Quantity)
		formattedOrders = append(formattedOrders, order)
	}

	return formattedOrders, err
}

package bbgo

import (
	"context"

	"github.com/pkg/errors"

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

	var formattedOrders []types.SubmitOrder
	for _, order := range orders {
		market, ok := es.Market(order.Symbol)
		if !ok {
			return nil, errors.Errorf("market is not defined: %s", order.Symbol)
		}

		order.Market = market
		order.PriceString = market.FormatPrice(order.Price)
		order.QuantityString = market.FormatVolume(order.Quantity)
		formattedOrders = append(formattedOrders, order)
	}

	// e.Notify(":memo: Submitting order to %s %s %s %s with quantity: %s", session, order.Symbol, order.Type, order.Side, order.QuantityString, order)

	return es.Exchange.SubmitOrders(ctx, formattedOrders...)
}

// ExchangeOrderExecutor is an order executor wrapper for single exchange instance.
type ExchangeOrderExecutor struct {
	Notifiability

	session *ExchangeSession
}

func (e *ExchangeOrderExecutor) Session() *ExchangeSession {
	return e.session
}

func (e *ExchangeOrderExecutor) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) ([]types.Order, error) {
	var formattedOrders []types.SubmitOrder
	for _, order := range orders {
		market, ok := e.session.Market(order.Symbol)
		if !ok {
			return nil, errors.Errorf("market is not defined: %s", order.Symbol)
		}

		order.Market = market
		order.PriceString = market.FormatPrice(order.Price)
		order.QuantityString = market.FormatVolume(order.Quantity)
		formattedOrders = append(formattedOrders, order)

		// e.Notify(":memo: Submitting %s %s %s order with quantity: %s", order.Symbol, order.Type, order.Side, order.QuantityString, order)
	}

	return e.session.Exchange.SubmitOrders(ctx, formattedOrders...)
}

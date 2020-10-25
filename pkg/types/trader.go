package types

import "context"

type OrderExecutor interface {
	SubmitOrders(ctx context.Context, orders ...SubmitOrder) error
}

type OrderExecutionRouter interface {
	// SubmitOrderTo submit order to a specific exchange session
	SubmitOrdersTo(ctx context.Context, session string, orders ...SubmitOrder) error
}

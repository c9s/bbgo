package types

import "context"

type OrderExecutor interface {
	SubmitOrder(ctx context.Context, order SubmitOrder) error
}

type OrderExecutionRouter interface {
	// SubmitOrderTo submit order to a specific exchange session
	SubmitOrderTo(ctx context.Context, session string, order SubmitOrder) error
}

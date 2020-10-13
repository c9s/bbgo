package types

import "context"

type OrderExecutor interface {
	SubmitOrder(ctx context.Context, order SubmitOrder)
}

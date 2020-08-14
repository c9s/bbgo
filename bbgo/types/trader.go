package types

import "context"

type Trader interface {
	SubmitOrder(ctx context.Context, order *SubmitOrder)
}

package types

import (
	"context"
	"fmt"
)

// CancelReplaceFallbackService is the minimal order-mutation contract needed
// by the non-native cancel-replace fallback. It intentionally excludes
// CancelReplace to avoid recursive fallback dispatch.
type CancelReplaceFallbackService interface {
	SubmitOrder(ctx context.Context, order SubmitOrder) (createdOrder *Order, err error)
	CancelOrders(ctx context.Context, orders ...Order) error
}

// CancelReplaceByCancelAndCreate implements the conservative fallback for
// venues without a native cancel-replace endpoint. It cancels the old order
// first, then creates the replacement. The two operations are not atomic; a
// caller must reconcile the resulting order state after an ambiguous error.
func CancelReplaceByCancelAndCreate(
	ctx context.Context,
	service CancelReplaceFallbackService,
	order Order,
) (*Order, error) {
	if service == nil {
		return nil, fmt.Errorf("cancel-replace fallback service is nil")
	}
	if err := service.CancelOrders(ctx, order); err != nil {
		return nil, fmt.Errorf("cancel-replace fallback cancel failed: %w", err)
	}

	createdOrder, err := service.SubmitOrder(ctx, order.SubmitOrder)
	if err != nil {
		return nil, fmt.Errorf("cancel-replace fallback create failed: %w", err)
	}
	if createdOrder == nil {
		return nil, fmt.Errorf("cancel-replace fallback create returned nil order")
	}
	return createdOrder, nil
}

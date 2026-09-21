package sandbox

import (
	"context"

	"github.com/c9s/bbgo/pkg/types"
)

var _ types.ExchangeTradeService = (*Exchange)(nil)

// CancelReplace uses the same cancel-then-create semantics as a venue without
// a native atomic cancel-replace endpoint.
func (e *Exchange) CancelReplace(ctx context.Context, cancelReplaceMode types.CancelReplaceModeType, order types.Order) (*types.Order, error) {
	_ = cancelReplaceMode
	return types.CancelReplaceByCancelAndCreate(ctx, e, order)
}

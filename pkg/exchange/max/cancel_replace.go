package max

import (
	"context"

	"github.com/c9s/bbgo/pkg/types"
)

var _ types.ExchangeTradeService = (*Exchange)(nil)

// CancelReplace falls back to cancel-then-create because MAX has no common
// atomic cancel-replace implementation exposed by this adapter.
func (e *Exchange) CancelReplace(ctx context.Context, cancelReplaceMode types.CancelReplaceModeType, order types.Order) (*types.Order, error) {
	_ = cancelReplaceMode
	return types.CancelReplaceByCancelAndCreate(ctx, e, order)
}

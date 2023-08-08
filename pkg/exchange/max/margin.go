package max

import (
	"context"
	"errors"
	"fmt"

	v3 "github.com/c9s/bbgo/pkg/exchange/max/maxapi/v3"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TransferMarginAccountAsset transfers the asset into/out from the margin account
//
// types.TransferIn => Spot to Margin
// types.TransferOut => Margin to Spot
//
// to call this method, you must set the IsMargin = true
func (e *Exchange) TransferMarginAccountAsset(ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection) error {
	if e.IsIsolatedMargin {
		return errors.New("isolated margin is not supported")
	}

	return e.transferCrossMarginAccountAsset(ctx, asset, amount, io)
}

// transferCrossMarginAccountAsset transfer asset to the cross margin account or to the main account
func (e *Exchange) transferCrossMarginAccountAsset(ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection) error {
	req := e.v3margin.NewMarginTransferRequest()
	req.Currency(toLocalCurrency(asset))
	req.Amount(amount.String())

	if io == types.TransferIn {
		req.Side(v3.MarginTransferSideIn)
	} else if io == types.TransferOut {
		req.Side(v3.MarginTransferSideOut)
	} else {
		return fmt.Errorf("unexpected transfer direction: %d given", io)
	}

	resp, err := req.Do(ctx)
	return logResponse(resp, err, req)
}

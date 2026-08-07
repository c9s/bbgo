package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// MarginAllAsset is a single entry of the margin asset catalog returned by
// GET /sapi/v1/margin/allAssets. It describes whether an asset can be borrowed
// or used as collateral (mortgage) on the cross-margin account.
type MarginAllAsset struct {
	AssetName      string           `json:"assetName"`
	AssetFullName  string           `json:"assetFullName"`
	IsBorrowable   bool             `json:"isBorrowable"`
	IsMortgageable bool             `json:"isMortgageable"`
	UserMinBorrow  fixedpoint.Value `json:"userMinBorrow"`
	UserMinRepay   fixedpoint.Value `json:"userMinRepay"`

	// DelistTime is the timestamp when the asset is delisted from margin (optional).
	DelistTime types.MillisecondTimestamp `json:"delistTime,omitempty"`
}

func (a *MarginAllAsset) MarginBorrowableAsset() types.MarginBorrowableAsset {
	return types.MarginBorrowableAsset{
		Asset:          a.AssetName,
		IsBorrowable:   a.IsBorrowable,
		IsMortgageable: a.IsMortgageable,
		MinBorrow:      a.UserMinBorrow,
		MinRepay:       a.UserMinRepay,
	}
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/allAssets" -type GetMarginAllAssetsRequest -responseType []MarginAllAsset
type GetMarginAllAssetsRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset *string `param:"asset"`
}

func (c *RestClient) NewGetMarginAllAssetsRequest() *GetMarginAllAssetsRequest {
	return &GetMarginAllAssetsRequest{client: c}
}

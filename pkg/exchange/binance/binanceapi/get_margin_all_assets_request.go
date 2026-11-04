package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type MarginAssetInfo struct {
	AssetFullName  string            `json:"assetFullName"`
	AssetName      string            `json:"assetName"`
	IsBorrowable   bool              `json:"isBorrowable"`
	IsMortgageable bool              `json:"isMortgageable"`
	UserMinBorrow  *fixedpoint.Value `json:"userMinBorrow"`
	UserMinRepay   fixedpoint.Value  `json:"userMinRepay"`
	DelistTime     int64             `json:"delistTime"`
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/allAssets" -type GetMarginAllAssetsRequest -responseType []MarginAssetInfo
type GetMarginAllAssetsRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset *string `param:"asset"`
}

func (c *RestClient) NewGetMarginAllAssetsRequest() *GetMarginAllAssetsRequest {
	return &GetMarginAllAssetsRequest{client: c}
}

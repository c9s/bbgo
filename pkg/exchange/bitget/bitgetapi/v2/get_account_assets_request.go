package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type AccountAsset struct {
	Coin           string                     `json:"coin"`
	Available      fixedpoint.Value           `json:"available"`
	Frozen         fixedpoint.Value           `json:"frozen"`
	Locked         fixedpoint.Value           `json:"locked"`
	LimitAvailable fixedpoint.Value           `json:"limitAvailable"`
	UpdatedTime    types.MillisecondTimestamp `json:"uTime"`
}

type AssetType string

const (
	AssetTypeHoldOnly AssetType = "hold_only"
	AssetTypeHAll     AssetType = "all"
)

//go:generate GetRequest -url "/api/v2/spot/account/assets" -type GetAccountAssetsRequest -responseDataType []AccountAsset
type GetAccountAssetsRequest struct {
	client requestgen.AuthenticatedAPIClient

	coin      *string   `param:"symbol,query"`
	assetType AssetType `param:"assetType,query"`
}

func (c *Client) NewGetAccountAssetsRequest() *GetAccountAssetsRequest {
	return &GetAccountAssetsRequest{client: c.Client}
}

package binanceapi

import (
	"github.com/c9s/requestgen"
)

type FuturesMultiAssetsModeResponse struct {
	MultiAssetsMargin bool `json:"multiAssetsMargin"`
}

//go:generate requestgen -method GET -url "/fapi/v1/multiAssetsMargin" -type FuturesGetMultiAssetsModeRequest -responseType FuturesMultiAssetsModeResponse
type FuturesGetMultiAssetsModeRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *FuturesRestClient) NewFuturesGetMultiAssetsModeRequest() *FuturesGetMultiAssetsModeRequest {
	return &FuturesGetMultiAssetsModeRequest{client: c}
}

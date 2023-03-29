package binanceapi

import (
	"github.com/c9s/requestgen"
)

type MultiAssetsMarginMode string

const (
	MultiAssetsMarginModeOn  MultiAssetsMarginMode = "true"
	MultiAssetsMarginModeOff MultiAssetsMarginMode = "false"
)

// Code 200 == success
type FuturesChangeMultiAssetsModeResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

//go:generate requestgen -method POST -url "/fapi/v1/multiAssetsMargin" -type FuturesChangeMultiAssetsModeRequest -responseType FuturesChangeMultiAssetsModeResponse
type FuturesChangeMultiAssetsModeRequest struct {
	client requestgen.AuthenticatedAPIClient

	multiAssetsMargin MultiAssetsMarginMode `param:"multiAssetsMargin"`
}

func (c *FuturesRestClient) NewFuturesChangeMultiAssetsModeRequest() *FuturesChangeMultiAssetsModeRequest {
	return &FuturesChangeMultiAssetsModeRequest{client: c}
}

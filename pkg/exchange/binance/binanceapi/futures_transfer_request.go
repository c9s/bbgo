package binanceapi

import "github.com/c9s/requestgen"

type FuturesTransferType string

const (
	FuturesTransferSpotToUsdtFutures FuturesTransferType = "MAIN_UMFUTURE"
	FuturesTransferUsdtFuturesToSpot FuturesTransferType = "UMFUTURE_MAIN"

	FuturesTransferSpotToCoinFutures FuturesTransferType = "MAIN_CMFUTURE"
	FuturesTransferCoinFuturesToSpot FuturesTransferType = "CMFUTURE_MAIN"
)

type FuturesTransferResponse struct {
	TranId int64 `json:"tranId"`
}

// doc: https://developers.binance.com/en/docs/catalog/core-trading-wallet/api/rest-api/asset#user-universal-transfer
//
//go:generate requestgen -method POST -url "/sapi/v1/asset/transfer" -type FuturesTransferRequest -responseType .FuturesTransferResponse
type FuturesTransferRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset string `param:"asset,required"`

	// amount is a decimal in string format
	amount string `param:"amount,required"`

	transferType FuturesTransferType `param:"type"`
}

func (c *RestClient) NewFuturesTransferRequest() *FuturesTransferRequest {
	return &FuturesTransferRequest{client: c}
}

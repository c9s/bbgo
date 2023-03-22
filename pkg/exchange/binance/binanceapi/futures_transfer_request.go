package binanceapi

import "github.com/c9s/requestgen"

type FuturesTransferType int

const (
	FuturesTransferSpotToUsdtFutures FuturesTransferType = 1
	FuturesTransferUsdtFuturesToSpot FuturesTransferType = 2

	FuturesTransferSpotToCoinFutures FuturesTransferType = 3
	FuturesTransferCoinFuturesToSpot FuturesTransferType = 4
)

type FuturesTransferResponse struct {
	TranId int64 `json:"tranId"`
}

//go:generate requestgen -method POST -url "/sapi/v1/futures/transfer" -type FuturesTransferRequest -responseType .FuturesTransferResponse
type FuturesTransferRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset string `param:"asset"`

	// amount is a decimal in string format
	amount string `param:"amount"`

	transferType FuturesTransferType `param:"type"`
}

func (c *RestClient) NewFuturesTransferRequest() *FuturesTransferRequest {
	return &FuturesTransferRequest{client: c}
}

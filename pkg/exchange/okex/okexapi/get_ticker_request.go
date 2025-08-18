package okexapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

//go:generate GetRequest -url "/api/v5/market/ticker" -type GetTickerRequest -responseDataType []MarketTicker
type GetTickerRequest struct {
	client requestgen.APIClient

	instId string `param:"instId,query"`
}

func (c *RestClient) NewGetTickerRequest() *GetTickerRequest {
	return &GetTickerRequest{
		client: c,
	}
}

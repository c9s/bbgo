package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"
)

type CancelOrderBySymbolResponse string

//go:generate GetRequest -url "/api/spot/v1/trade/cancel-symbol-order" -type CancelOrderBySymbolRequest -responseDataType .CancelOrderBySymbolResponse
type CancelOrderBySymbolRequest struct {
	client requestgen.AuthenticatedAPIClient
	symbol string `param:"symbol"`
}

func (c *RestClient) NewCancelOrderBySymbolRequest() *CancelOrderBySymbolRequest {
	return &CancelOrderBySymbolRequest{client: c}
}

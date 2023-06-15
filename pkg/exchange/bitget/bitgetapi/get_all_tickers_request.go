package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"
)

//go:generate GetRequest -url "/api/spot/v1/market/tickers" -type GetAllTickersRequest -responseDataType []Ticker
type GetAllTickersRequest struct {
	client requestgen.APIClient
}

func (c *RestClient) NewGetAllTickersRequest() *GetAllTickersRequest {
	return &GetAllTickersRequest{client: c}
}

package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"
)

//go:generate GetRequest -url "/api/spot/v1/trade/open-orders" -type GetOpenOrdersRequest -responseDataType []OrderDetail
type GetOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol"`
}

func (c *RestClient) NewGetOpenOrdersRequest() *GetOpenOrdersRequest {
	return &GetOpenOrdersRequest{client: c}
}

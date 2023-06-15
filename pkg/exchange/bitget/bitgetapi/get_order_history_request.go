package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"
)

//go:generate GetRequest -url "/api/spot/v1/trade/history" -type GetOrderHistoryRequest -responseDataType []OrderDetail
type GetOrderHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol"`

	// after - order id
	after *string `param:"after"`

	// before - order id
	before *string `param:"before"`

	limit *string `param:"limit"`
}

func (c *RestClient) NewGetOrderHistoryRequest() *GetOrderHistoryRequest {
	return &GetOrderHistoryRequest{client: c}
}

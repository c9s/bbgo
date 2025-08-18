package okexapi

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

//go:generate PostRequest -url "/api/v5/trade/cancel-order" -type CancelOrderRequest -responseDataType []OrderResponse
type CancelOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentID  string  `param:"instId"`
	orderID       *string `param:"ordId"`
	clientOrderID *string `param:"clOrdId"`
}

func (c *RestClient) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{
		client: c,
	}
}

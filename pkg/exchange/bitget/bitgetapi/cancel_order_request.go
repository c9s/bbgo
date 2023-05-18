package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"
)

type CancelOrderResponse struct {
	OrderId       string `json:"orderId"`
	ClientOrderId string `json:"clientOrderId"`
}

//go:generate PostRequest -url "/api/spot/v1/trade/cancel-order-v2" -type CancelOrderRequest -responseDataType .CancelOrderResponse
type CancelOrderRequest struct {
	client        requestgen.AuthenticatedAPIClient
	symbol        string  `param:"symbol"`
	orderId       *string `param:"orderId"`
	clientOrderId *string `param:"clientOid"`
}

func (c *RestClient) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{client: c}
}

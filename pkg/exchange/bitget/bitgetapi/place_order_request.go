package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"
)

type OrderResponse struct {
	OrderId       string `json:"orderId"`
	ClientOrderId string `json:"clientOrderId"`
}

//go:generate PostRequest -url "/api/spot/v1/trade/orders" -type PlaceOrderRequest -responseDataType .OrderResponse
type PlaceOrderRequest struct {
	client        requestgen.AuthenticatedAPIClient
	symbol        string     `param:"symbol"`
	orderType     OrderType  `param:"orderType"`
	side          OrderSide  `param:"side"`
	force         OrderForce `param:"force"`
	price         string     `param:"price"`
	quantity      string     `param:"quantity"`
	clientOrderId *string    `param:"clientOrderId"`
}

func (c *RestClient) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{client: c}
}

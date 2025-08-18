package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"
)

type PlaceOrderResponse struct {
	OrderId       string `json:"orderId"`
	ClientOrderId string `json:"clientOid"`
}

//go:generate PostRequest -url "/api/v2/spot/trade/place-order" -type PlaceOrderRequest -responseDataType .PlaceOrderResponse
type PlaceOrderRequest struct {
	client        requestgen.AuthenticatedAPIClient
	symbol        string     `param:"symbol"`
	orderType     OrderType  `param:"orderType"`
	side          SideType   `param:"side"`
	force         OrderForce `param:"force"`
	price         *string    `param:"price"`
	size          string     `param:"size"`
	clientOrderId *string    `param:"clientOid"`
}

func (c *Client) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{client: c.Client}
}

package kucoinapi

import "github.com/c9s/requestgen"

//go:generate requestgen -method GET -url /api/v1/orders/{orderId} -type GetOrderRequest -responseType .Order
type GetOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	orderId string `param:"orderId,slug,required"`
}

func (c *TradeService) NewGetOrderRequest() *GetOrderRequest {
	return &GetOrderRequest{
		client: c.client,
	}
}

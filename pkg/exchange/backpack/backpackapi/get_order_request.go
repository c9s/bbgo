package backpackapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET

// GetOrderRequest queries a single resting order.
//
// Exactly one of orderId or clientId must be set. The API documents that when both are
// provided orderId takes precedence and the signature verification fails, so setting both is
// always a bug.
//
//go:generate GetRequest -url "/api/v1/order" -type GetOrderRequest -responseType .Order
type GetOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol,required"`

	orderId  *string `param:"orderId"`
	clientId *uint32 `param:"clientId"`
}

func (c *RestClient) NewGetOrderRequest() *GetOrderRequest {
	return &GetOrderRequest{client: c.withInstruction(InstructionOrderQuery)}
}

//go:generate GetRequest -url "/api/v1/orders" -type GetOpenOrdersRequest -responseType []Order
type GetOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol     *string     `param:"symbol"`
	marketType *MarketType `param:"marketType"`
}

func (c *RestClient) NewGetOpenOrdersRequest() *GetOpenOrdersRequest {
	return &GetOpenOrdersRequest{client: c.withInstruction(InstructionOrderQueryAll)}
}

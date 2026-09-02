package backpackapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command DeleteRequest requestgen -method DELETE

// CancelOrderRequest cancels a single resting order.
//
// Exactly one of orderId or clientId must be set; see GetOrderRequest for why setting both
// breaks the signature.
//
//go:generate DeleteRequest -url "/api/v1/order" -type CancelOrderRequest -responseType .Order
type CancelOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol,required"`

	orderId  *string `param:"orderId"`
	clientId *uint32 `param:"clientId"`
}

func (c *RestClient) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{client: c.withInstruction(InstructionOrderCancel)}
}

//go:generate DeleteRequest -url "/api/v1/orders" -type CancelAllOrdersRequest -responseType []Order
type CancelAllOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol,required"`

	orderType *CancelOrderType `param:"orderType"`
}

func (c *RestClient) NewCancelAllOrdersRequest() *CancelAllOrdersRequest {
	return &CancelAllOrdersRequest{client: c.withInstruction(InstructionOrderCancelAll)}
}

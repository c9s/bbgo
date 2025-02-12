package coinbase

import "github.com/c9s/requestgen"

type CancelOrderResponse string

//go:generate requestgen -method DELETE -url "/orders/:order_id" -type CancelOrderRequest -responseType .CancelOrderResponse
type CancelOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	orderID string `param:"order_id,slug,required"`

	profileID *string `param:"profile_id"`
	productID *string `param:"product_id"`
}

func (c *RestAPIClient) NewCancelOrderRequest(orderID string) *CancelOrderRequest {
	return &CancelOrderRequest{client: c, orderID: orderID}
}

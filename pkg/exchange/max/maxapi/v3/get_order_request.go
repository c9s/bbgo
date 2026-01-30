package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import "github.com/c9s/requestgen"

func (c *Client) NewGetOrderRequest() *GetOrderRequest {
	return &GetOrderRequest{client: c}
}

//go:generate GetRequest -url "/api/v3/order" -type GetOrderRequest -responseType .Order
type GetOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	id            *uint64 `param:"id,omitempty"`
	clientOrderID *string `param:"client_oid,omitempty"`
}

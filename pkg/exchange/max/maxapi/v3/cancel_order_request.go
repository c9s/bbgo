package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import "github.com/c9s/requestgen"

func (s *OrderService) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{client: s.Client}
}

//go:generate DeleteRequest -url "/api/v3/order" -type CancelOrderRequest -responseType .Order
type CancelOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	id            *uint64 `param:"id,omitempty"`
	clientOrderID *string `param:"client_oid,omitempty"`
}

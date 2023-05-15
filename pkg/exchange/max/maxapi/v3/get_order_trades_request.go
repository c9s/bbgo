package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import "github.com/c9s/requestgen"

func (s *Client) NewGetOrderTradesRequest() *GetOrderTradesRequest {
	return &GetOrderTradesRequest{client: s.Client}
}

//go:generate GetRequest -url "/api/v3/order/trades" -type GetOrderTradesRequest -responseType []Trade
type GetOrderTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	orderID       *uint64 `param:"order_id,omitempty"`
	clientOrderID *string `param:"client_oid,omitempty"`
}

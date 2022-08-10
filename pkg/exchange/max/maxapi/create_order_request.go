package max

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

//go:generate PostRequest -url "/api/v2/orders" -type CreateOrderRequest -responseType .Order
type CreateOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	market    string    `param:"market,required"`
	side      string    `param:"side,required"`
	volume    string    `param:"volume,required"`
	orderType OrderType `param:"ord_type"`

	price         *string `param:"price"`
	stopPrice     *string `param:"stop_price"`
	clientOrderID *string `param:"client_oid"`
	groupID       *string `param:"group_id"`
}

func (s *OrderService) NewCreateOrderRequest() *CreateOrderRequest {
	return &CreateOrderRequest{client: s.client}
}

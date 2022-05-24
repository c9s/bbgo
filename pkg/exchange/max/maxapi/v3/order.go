package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"github.com/c9s/requestgen"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
)

type Order = maxapi.Order

// OrderService manages the Order endpoint.
type OrderService struct {
	client *maxapi.RestClient
}

func (s *OrderService) NewCreateOrderRequest() *CreateOrderRequest {
	return &CreateOrderRequest{client: s.client}
}

func (s *OrderService) NewGetOrderRequest() *GetOrderRequest {
	return &GetOrderRequest{client: s.client}
}

func (s *OrderService) NewOrderCancelAllRequest() *OrderCancelAllRequest {
	return &OrderCancelAllRequest{client: s.client}
}

func (s *OrderService) NewOrderCancelRequest() *OrderCancelRequest {
	return &OrderCancelRequest{client: s.client}
}

//go:generate PostRequest -url "/api/v3/wallet/:walletType/orders" -type CreateOrderRequest -responseType .Order -debug
type CreateOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType string `param:"walletType,slug,required"`
	market     string `param:"market,required"`
	side       string `param:"side,required"`
	volume     string `param:"volume,required"`
	orderType  string `param:"ord_type"`

	price         *string `param:"price"`
	stopPrice     *string `param:"stop_price"`
	clientOrderID *string `param:"client_oid"`
	groupID       *string `param:"group_id"`
}

//go:generate GetRequest -url "/api/v3/wallet/:walletType/orders" -type GetOrderRequest -responseType .Order
type GetOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType string `param:"walletType,slug,required"`
	market     string `param:"market,required"`
	side       string `param:"side,required"`
	volume     string `param:"volume,required"`
	orderType  string `param:"ord_type"`

	price         *string `param:"price"`
	stopPrice     *string `param:"stop_price"`
	clientOrderID *string `param:"client_oid"`
	groupID       *string `param:"group_id"`
}

//go:generate DeleteRequest -url "/api/v3/wallet/:walletType/orders" -type OrderCancelAllRequest -responseType []Order
type OrderCancelAllRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType string  `param:"walletType,slug,required"`
	side       *string `param:"side"`
	market     *string `param:"market"`
	groupID    *uint32 `param:"groupID"`
}

//go:generate PostRequest -url "/api/v3/order" -type OrderCancelRequest -responseType .Order
type OrderCancelRequest struct {
	client requestgen.AuthenticatedAPIClient

	id            *uint64 `param:"id,omitempty"`
	clientOrderID *string `param:"client_oid,omitempty"`
}

package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"time"

	"github.com/c9s/requestgen"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
)

type WalletType = maxapi.WalletType

type Order = maxapi.Order

// OrderService manages the Order endpoint.
type OrderService struct {
	Client *maxapi.RestClient
}

func (s *OrderService) NewWalletCreateOrderRequest(walletType WalletType) *WalletCreateOrderRequest {
	return &WalletCreateOrderRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewWalletGetOrderHistoryRequest(walletType WalletType) *WalletGetOrderHistoryRequest {
	return &WalletGetOrderHistoryRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewWalletGetOpenOrdersRequest(walletType WalletType) *WalletGetOpenOrdersRequest {
	return &WalletGetOpenOrdersRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewWalletOrderCancelAllRequest(walletType WalletType) *WalletOrderCancelAllRequest {
	return &WalletOrderCancelAllRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewWalletGetTradesRequest(walletType WalletType) *WalletGetTradesRequest {
	return &WalletGetTradesRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewOrderCancelRequest() *OrderCancelRequest {
	return &OrderCancelRequest{client: s.Client}
}

func (s *OrderService) NewGetOrderRequest() *GetOrderRequest {
	return &GetOrderRequest{client: s.Client}
}

//go:generate PostRequest -url "/api/v3/wallet/:walletType/orders" -type WalletCreateOrderRequest -responseType .Order -debug
type WalletCreateOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`
	market     string     `param:"market,required"`
	side       string     `param:"side,required"`
	volume     string     `param:"volume,required"`
	orderType  string     `param:"ord_type"`

	price         *string `param:"price"`
	stopPrice     *string `param:"stop_price"`
	clientOrderID *string `param:"client_oid"`
	groupID       *string `param:"group_id"`
}

//go:generate GetRequest -url "/api/v3/wallet/:walletType/orders/history" -type WalletGetOrderHistoryRequest -responseType []Order
type WalletGetOrderHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`

	market string  `param:"market,required"`
	fromID *uint64 `param:"from_id"`
	limit  *uint   `param:"limit"`
}

//go:generate GetRequest -url "/api/v3/wallet/:walletType/orders/open" -type WalletGetOpenOrdersRequest -responseType []Order
type WalletGetOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`
	market     string     `param:"market,required"`
}

//go:generate DeleteRequest -url "/api/v3/wallet/:walletType/orders" -type WalletOrderCancelAllRequest -responseType []Order
type WalletOrderCancelAllRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`
	side       *string    `param:"side"`
	market     *string    `param:"market"`
	groupID    *uint32    `param:"groupID"`
}

type Trade = maxapi.Trade

//go:generate GetRequest -url "/api/v3/wallet/:walletType/trades" -type WalletGetTradesRequest -responseType []Trade
type WalletGetTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`

	market    string     `param:"market,required"`
	from      *uint64    `param:"from_id"`
	startTime *time.Time `param:"start_time,milliseconds"`
	endTime   *time.Time `param:"end_time,milliseconds"`
	limit     *uint64    `param:"limit"`
}

//go:generate PostRequest -url "/api/v3/order" -type OrderCancelRequest -responseType .Order
type OrderCancelRequest struct {
	client requestgen.AuthenticatedAPIClient

	id            *uint64 `param:"id,omitempty"`
	clientOrderID *string `param:"client_oid,omitempty"`
}

//go:generate GetRequest -url "/api/v3/order" -type GetOrderRequest -responseType .Order
type GetOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	id            *uint64 `param:"id,omitempty"`
	clientOrderID *string `param:"client_oid,omitempty"`
}

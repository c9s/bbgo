package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"time"

	"github.com/c9s/requestgen"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
)

// create type alias
type WalletType = maxapi.WalletType
type Order = maxapi.Order
type Trade = maxapi.Trade
type Account = maxapi.Account

// OrderService manages the Order endpoint.
type OrderService struct {
	Client *maxapi.RestClient
}

func (s *OrderService) NewWalletCreateOrderRequest(walletType WalletType) *CreateWalletOrderRequest {
	return &CreateWalletOrderRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewWalletGetOrderHistoryRequest(walletType WalletType) *GetWalletOrderHistoryRequest {
	return &GetWalletOrderHistoryRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewWalletGetOpenOrdersRequest(walletType WalletType) *GetWalletOpenOrdersRequest {
	return &GetWalletOpenOrdersRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewWalletOrderCancelAllRequest(walletType WalletType) *CancelWalletOrderAllRequest {
	return &CancelWalletOrderAllRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewWalletGetTradesRequest(walletType WalletType) *GetWalletTradesRequest {
	return &GetWalletTradesRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewOrderCancelRequest() *CancelOrderRequest {
	return &CancelOrderRequest{client: s.Client}
}

func (s *OrderService) NewGetOrderRequest() *GetOrderRequest {
	return &GetOrderRequest{client: s.Client}
}

//go:generate GetRequest -url "/api/v3/wallet/:walletType/accounts" -type GetWalletAccountsRequest -responseType []Account
type GetWalletAccountsRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`
}

//go:generate PostRequest -url "/api/v3/wallet/:walletType/orders" -type CreateWalletOrderRequest -responseType .Order
type CreateWalletOrderRequest struct {
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

//go:generate GetRequest -url "/api/v3/wallet/:walletType/orders/history" -type GetWalletOrderHistoryRequest -responseType []Order
type GetWalletOrderHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`

	market string  `param:"market,required"`
	fromID *uint64 `param:"from_id"`
	limit  *uint   `param:"limit"`
}

//go:generate GetRequest -url "/api/v3/wallet/:walletType/orders/open" -type GetWalletOpenOrdersRequest -responseType []Order
type GetWalletOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`
	market     string     `param:"market,required"`
}

//go:generate DeleteRequest -url "/api/v3/wallet/:walletType/orders" -type CancelWalletOrderAllRequest -responseType []Order
type CancelWalletOrderAllRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`
	side       *string    `param:"side"`
	market     *string    `param:"market"`
	groupID    *uint32    `param:"groupID"`
}

//go:generate GetRequest -url "/api/v3/wallet/:walletType/trades" -type GetWalletTradesRequest -responseType []Trade
type GetWalletTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`

	market    string     `param:"market,required"`
	from      *uint64    `param:"from_id"`
	startTime *time.Time `param:"start_time,milliseconds"`
	endTime   *time.Time `param:"end_time,milliseconds"`
	limit     *uint64    `param:"limit"`
}

//go:generate PostRequest -url "/api/v3/order" -type CancelOrderRequest -responseType .Order
type CancelOrderRequest struct {
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

package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
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

func (s *OrderService) NewGetWalletAccountsRequest(walletType WalletType) *GetWalletAccountsRequest {
	return &GetWalletAccountsRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewCreateWalletOrderRequest(walletType WalletType) *CreateWalletOrderRequest {
	return &CreateWalletOrderRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewGetWalletOrderHistoryRequest(walletType WalletType) *GetWalletOrderHistoryRequest {
	return &GetWalletOrderHistoryRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewGetWalletOpenOrdersRequest(walletType WalletType) *GetWalletOpenOrdersRequest {
	return &GetWalletOpenOrdersRequest{client: s.Client, walletType: walletType}
}

func (s *OrderService) NewCancelWalletOrderAllRequest(walletType WalletType) *CancelWalletOrderAllRequest {
	return &CancelWalletOrderAllRequest{client: s.Client, walletType: walletType}
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


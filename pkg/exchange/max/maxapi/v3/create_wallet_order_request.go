package v3

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

//go:generate PostRequest -url "/api/v3/wallet/:walletType/order" -type CreateWalletOrderRequest -responseType .Order
type CreateWalletOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`
	market     string     `param:"market,required"`
	side       string     `param:"side,required"`
	volume     string     `param:"volume,required"`
	orderType  OrderType     `param:"ord_type"`

	price         *string `param:"price"`
	stopPrice     *string `param:"stop_price"`
	clientOrderID *string `param:"client_oid"`
	groupID       *string `param:"group_id"`
}

func (s *OrderService) NewCreateWalletOrderRequest(walletType WalletType) *CreateWalletOrderRequest {
	return &CreateWalletOrderRequest{client: s.Client, walletType: walletType}
}

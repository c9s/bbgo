package v3

import "github.com/c9s/requestgen"

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

func (s *OrderService) NewCreateWalletOrderRequest(walletType WalletType) *CreateWalletOrderRequest {
	return &CreateWalletOrderRequest{client: s.Client, walletType: walletType}
}

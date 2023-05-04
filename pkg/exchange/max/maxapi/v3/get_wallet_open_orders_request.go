package v3

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

func (s *Client) NewGetWalletOpenOrdersRequest(walletType WalletType) *GetWalletOpenOrdersRequest {
	return &GetWalletOpenOrdersRequest{client: s.Client, walletType: walletType}
}

//go:generate GetRequest -url "/api/v3/wallet/:walletType/orders/open" -type GetWalletOpenOrdersRequest -responseType []Order
type GetWalletOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`
	market     string     `param:"market,required"`
}

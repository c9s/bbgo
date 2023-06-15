package v3

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

//go:generate GetRequest -url "/api/v3/wallet/:walletType/accounts" -type GetWalletAccountsRequest -responseType []Account
type GetWalletAccountsRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`
	currency   *string    `param:"currency,query"`
}

func (s *Client) NewGetWalletAccountsRequest(walletType WalletType) *GetWalletAccountsRequest {
	return &GetWalletAccountsRequest{client: s.Client, walletType: walletType}
}

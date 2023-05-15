package v3

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

func (s *Client) NewGetWalletOrderHistoryRequest(walletType WalletType) *GetWalletOrderHistoryRequest {
	return &GetWalletOrderHistoryRequest{client: s.Client, walletType: walletType}
}

//go:generate GetRequest -url "/api/v3/wallet/:walletType/orders/history" -type GetWalletOrderHistoryRequest -responseType []Order
type GetWalletOrderHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`

	market string  `param:"market,required"`
	fromID *uint64 `param:"from_id"`
	limit  *uint   `param:"limit"`
}

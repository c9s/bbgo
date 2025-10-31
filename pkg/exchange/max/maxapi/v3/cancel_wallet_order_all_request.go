package v3

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

//go:generate DeleteRequest -url "/api/v3/wallet/:walletType/orders" -type CancelWalletOrderAllRequest -responseType []OrderCancelResponse -debug
type CancelWalletOrderAllRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`
	side       *string    `param:"side"`
	market     *string    `param:"market"`
	groupID    *uint32    `param:"group_id"`
}

type OrderCancelResponse struct {
	Order Order
	Error *string
}

func (c *Client) NewCancelWalletOrderAllRequest(walletType WalletType) *CancelWalletOrderAllRequest {
	return &CancelWalletOrderAllRequest{client: c.RestClient, walletType: walletType}
}

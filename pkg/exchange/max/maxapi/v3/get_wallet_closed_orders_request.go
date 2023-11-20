package v3

import (
	"time"

	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

func (s *Client) NewGetWalletClosedOrdersRequest(walletType WalletType) *GetWalletClosedOrdersRequest {
	return &GetWalletClosedOrdersRequest{client: s.Client, walletType: walletType}
}

//go:generate GetRequest -url "/api/v3/wallet/:walletType/orders/closed" -type GetWalletClosedOrdersRequest -responseType []Order
type GetWalletClosedOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`

	market    string       `param:"market,required"`
	timestamp *time.Time   `param:"timestamp,milliseconds,omitempty"`
	orderBy   *OrderByType `param:"order_by,omitempty"`
	limit     *uint        `param:"limit,omitempty"`
}

package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"time"

	"github.com/c9s/requestgen"
)

func (s *Client) NewGetWalletTradesRequest(walletType WalletType) *GetWalletTradesRequest {
	return &GetWalletTradesRequest{client: s.Client, walletType: walletType}
}

//go:generate GetRequest -url "/api/v3/wallet/:walletType/trades" -type GetWalletTradesRequest -responseType []Trade
type GetWalletTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	walletType WalletType `param:"walletType,slug,required"`

	market    string     `param:"market,required"`
	timestamp *time.Time `param:"timestamp,milliseconds,omitempty"`
	fromID    *uint64    `param:"from_id,omitempty"`
	order     *string    `param:"order,omitempty" validValues:"asc,desc"`
	limit     *uint64    `param:"limit"`
}

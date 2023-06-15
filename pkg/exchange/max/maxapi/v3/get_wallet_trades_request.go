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
	from      *uint64    `param:"from_id"`
	startTime *time.Time `param:"start_time,milliseconds"`
	endTime   *time.Time `param:"end_time,milliseconds"`
	limit     *uint64    `param:"limit"`
}

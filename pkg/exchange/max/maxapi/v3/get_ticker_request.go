package v3

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

//go:generate GetRequest -url "/api/v3/tickers/:market" -type GetTickerRequest -responseType .Ticker
type GetTickerRequest struct {
	client requestgen.APIClient

	market *string `param:"market,slug"`
}

func (c *Client) NewGetTickerRequest() *GetTickerRequest {
	return &GetTickerRequest{client: c.Client}
}

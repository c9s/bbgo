package max

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

//go:generate GetRequest -url "/api/v2/tickers/:market" -type GetTickerRequest -responseType .Ticker
type GetTickerRequest struct {
	client requestgen.APIClient

	market *string `param:"market,slug"`
}

func (c *RestClient) NewGetTickerRequest() *GetTickerRequest {
	return &GetTickerRequest{client: c}
}

package max

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

type TickerMap map[string]Ticker

//go:generate GetRequest -url "/api/v2/tickers" -type GetTickersRequest -responseType .TickerMap
type GetTickersRequest struct {
	client requestgen.APIClient
}

func (c *RestClient) NewGetTickersRequest() *GetTickersRequest {
	return &GetTickersRequest{client: c}
}

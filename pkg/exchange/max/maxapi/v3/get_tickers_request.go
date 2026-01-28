package v3

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

//go:generate GetRequest -url "/api/v3/tickers" -type GetTickersRequest -responseType []Ticker
type GetTickersRequest struct {
	client requestgen.APIClient

	markets []string `param:"markets,query,omitempty"` // list of markets of tickers
}

func (c *Client) NewGetTickersRequest() *GetTickersRequest {
	return &GetTickersRequest{client: c}
}

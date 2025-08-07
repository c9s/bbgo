package bfxapi

import (
	"github.com/c9s/requestgen"
)

// GetCandlesRequest represents a request for Bitfinex public candles endpoint.
//
//go:generate requestgen -type GetCandlesRequest -method GET -url "/v2/candles/:candle/:section" -responseType .CandlesResponse
type GetCandlesRequest struct {
	client requestgen.APIClient

	candle string `param:"candle,slug"` // e.g. tBTCUSD

	section string `param:"section,slug" validValues:"last,hist"` // e.g. last, hist

	sort  int   `param:"sort"`  // optional, 1 for ascending, -1 for descending order
	start int64 `param:"start"` // optional, millisecond timestamp
	end   int64 `param:"end"`   // optional, millisecond timestamp

	limit int `param:"limit"` // optional, max number of candles
}

// NewGetCandlesRequest creates a new GetCandlesRequest.
func (c *Client) NewGetCandlesRequest() *GetCandlesRequest {
	return &GetCandlesRequest{client: c}
}

// CandlesResponse represents multiple candles response.
type CandlesResponse []Candle

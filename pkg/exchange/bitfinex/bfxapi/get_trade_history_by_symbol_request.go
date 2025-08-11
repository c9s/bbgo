package bfxapi

import (
	"time"

	"github.com/c9s/requestgen"
)

// GetTradeHistoryBySymbolRequest
// API: https://docs.bitfinex.com/reference/rest-auth-trades-by-symbol
//
//go:generate requestgen -type GetTradeHistoryBySymbolRequest -method POST -url "/v2/auth/r/trades/:symbol/hist" -responseType []OrderTradeDetail
type GetTradeHistoryBySymbolRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string     `param:"symbol,slug"`          // Trading pair symbol (e.g., "tBTCUSD")
	start  *time.Time `param:"start,milliseconds"`   // Start timestamp in ms
	end    *time.Time `param:"end,milliseconds"`     // End timestamp in ms
	limit  *int       `param:"limit" default:"2500"` // Limit number of results

	sort *int `param:"sort" default:"-1"` // Sort order: -1 for descending, 1 for ascending
}

// NewGetTradeHistoryBySymbolRequest creates a new GetTradeHistoryBySymbolRequest.
func (c *Client) NewGetTradeHistoryBySymbolRequest() *GetTradeHistoryBySymbolRequest {
	return &GetTradeHistoryBySymbolRequest{client: c}
}

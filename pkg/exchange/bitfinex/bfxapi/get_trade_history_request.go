package bfxapi

import (
	"time"

	"github.com/c9s/requestgen"
)

// GetTradeHistoryRequest
// API: https://docs.bitfinex.com/reference/rest-auth-trades
//
//go:generate requestgen -type GetTradeHistoryRequest -method POST -url "/v2/auth/r/trades/hist" -responseType []OrderTradeDetail
type GetTradeHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	start *time.Time `param:"start,milliseconds"`   // Start timestamp in ms
	end   *time.Time `param:"end,milliseconds"`     // End timestamp in ms
	limit *int       `param:"limit" default:"2500"` // Limit number of results

	sort *int `param:"sort" default:"-1"` // Sort order: -1 for descending, 1 for ascending
}

// NewGetTradeHistoryRequest creates a new GetTradeHistoryRequest
func (c *Client) NewGetTradeHistoryRequest() *GetTradeHistoryRequest {
	return &GetTradeHistoryRequest{client: c}
}

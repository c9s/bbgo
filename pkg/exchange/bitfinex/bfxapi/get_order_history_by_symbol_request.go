package bfxapi

import (
	"time"

	"github.com/c9s/requestgen"
)

// GetOrderHistoryBySymbolRequest retrieves all user's closed/cancelled orders up to 2 weeks in the past by trading pair symbol (e.g. tBTCUSD, tLTCBTC, ...).
// API: https://docs.bitfinex.com/reference/rest-auth-orders-history-by-symbol
//
//go:generate requestgen -type GetOrderHistoryBySymbolRequest -method POST -url "/v2/auth/r/orders/:symbol/hist" -responseType []OrderData
type GetOrderHistoryBySymbolRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string     `param:"symbol,slug"`          // Trading pair symbol (e.g., "tBTCUSD")
	start  *time.Time `param:"start,milliseconds"`   // Start timestamp in ms
	end    *time.Time `param:"end,milliseconds"`     // End timestamp in ms
	limit  *int       `param:"limit" default:"2500"` // Limit number of results

	orderId []int64 `param:"id,omitempty"` // Order ID to filter results
}

// NewGetOrderHistoryRequest creates a new GetOrderHistoryBySymbolRequest.
func (c *Client) NewGetOrderHistoryBySymbolRequest() *GetOrderHistoryBySymbolRequest {
	return &GetOrderHistoryBySymbolRequest{client: c}
}

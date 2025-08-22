package bfxapi

import (
	"github.com/c9s/requestgen"
)

// API: https://docs.bitfinex.com/reference/rest-auth-retrieve-orders-by-symbol
// RetrieveOrderBySymbolRequest gets all the current user's active orders.
//
//go:generate requestgen -type RetrieveOrderBySymbolRequest -method POST -url "/v2/auth/r/orders/:symbol" -responseType .RetrieveOrderResponse
type RetrieveOrderBySymbolRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol,slug"` // e.g. tBTCUSD, tETHUSD, etc.

	// id allows retrieving specific orders by order ID
	id []int64 `param:"id,omitempty"`

	// gid filters results based on Group ID
	gid *int64 `param:"gid,omitempty"`

	// cid filters based on Client ID (requires cid_date)
	cid *string `param:"cid,omitempty"`

	// cid_date must be provided with cid, format: "YYYY-MM-DD"
	cidDate *string `param:"cid_date,omitempty"`
}

// NewRetrieveOrderRequest creates a new RetrieveOrderRequest.
func (c *Client) NewRetrieveOrderBySymbolRequest() *RetrieveOrderBySymbolRequest {
	return &RetrieveOrderBySymbolRequest{client: c}
}

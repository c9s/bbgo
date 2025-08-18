package bfxapi

import (
	"github.com/c9s/requestgen"
)

// GetOrderTradesRequest represents a request for Bitfinex order trades API.
// API: https://docs.bitfinex.com/reference/rest-auth-order-trades
//
//go:generate requestgen -type GetOrderTradesRequest -method POST -url "/v2/auth/r/order/:symbol::id/trades" -responseType []OrderTradeDetail
type GetOrderTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol,slug,required"` // trading pair symbol (e.g., "tETHUSD")
	id     int64  `param:"id,slug,required"`     // order ID
}

// NewGetOrderTradesRequest creates a new GetOrderTradesRequest instance.
func (c *Client) NewGetOrderTradesRequest() *GetOrderTradesRequest {
	return &GetOrderTradesRequest{client: c}
}

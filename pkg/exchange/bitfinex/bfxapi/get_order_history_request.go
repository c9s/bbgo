package bfxapi

import (
	"time"

	"github.com/c9s/requestgen"
)

// GetOrderHistoryRequest used to retrieve the order history for a specific trading pair on Bitfinex.
// API: https://docs.bitfinex.com/reference/rest-auth-orders-history
//
//go:generate requestgen -type GetOrderHistoryRequest -method POST -url "/v2/auth/r/orders" -responseType []OrderData
type GetOrderHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	start *time.Time `param:"start,milliseconds"`   // Start timestamp in ms
	end   *time.Time `param:"end,milliseconds"`     // End timestamp in ms
	limit *int       `param:"limit" default:"2500"` // Limit number of results

	orderId []int64 `param:"id,omitempty"` // Order ID to filter results
}

// NewGetOrderHistoryRequest 创建一个新的订单历史请求。
func (c *Client) NewGetOrderHistoryRequest() *GetOrderHistoryRequest {
	return &GetOrderHistoryRequest{client: c}
}

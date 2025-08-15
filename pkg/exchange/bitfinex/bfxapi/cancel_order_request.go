package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

// API: https://docs.bitfinex.com/reference/rest-auth-cancel-order
// CancelOrderRequest represents a Bitfinex order cancellation request.
//
// CancelOrderRequest is used to cancel an order by ID, GID, or CID.
//
//go:generate requestgen -type CancelOrderRequest -method POST -url "/v2/auth/w/order/cancel" -responseType .CancelOrderResponse
type CancelOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	// orderID is the order ID to cancel
	orderID *int64 `param:"id,omitempty"`

	// groupID is the group ID to cancel
	groupID *int64 `param:"gid,omitempty"`

	// clientOrderID is the client order ID to cancel
	clientOrderID *int64 `param:"cid,omitempty"`

	// cid_date is required if cid is used, format: "YYYY-MM-DD"
	clientOrderIDDate string `param:"cid_date,omitempty"`
}

// NewCancelOrderRequest creates a new CancelOrderRequest.
func (c *Client) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{client: c}
}

// CancelOrderResponse represents the response from Bitfinex order cancellation.
type CancelOrderResponse struct {
	Time      types.MillisecondTimestamp
	Type      string
	MessageID *int
	_         any
	Data      Order
	Code      *int64
	Status    string
	Text      string
}

// UnmarshalJSON parses the Bitfinex array response into CancelOrderResponse.
func (r *CancelOrderResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

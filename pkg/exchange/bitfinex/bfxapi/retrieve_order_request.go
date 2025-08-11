package bfxapi

import (
	"encoding/json"

	"github.com/c9s/requestgen"
)

// API: https://docs.bitfinex.com/reference/rest-auth-retrieve-orders
// RetrieveOrderRequest gets all the current user's active orders.
//
//go:generate requestgen -type RetrieveOrderRequest -method POST -url "/v2/auth/r/orders" -responseType .RetrieveOrderResponse
type RetrieveOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	// id allows retrieving specific orders by order ID
	id []int64 `param:"id,omitempty"`

	// gid filters results based on Group ID
	gid int64 `param:"gid,omitempty"`

	// cid filters based on Client ID (requires cid_date)
	cid string `param:"cid,omitempty"`

	// cid_date must be provided with cid, format: "YYYY-MM-DD"
	cidDate string `param:"cid_date,omitempty"`
}

// NewRetrieveOrderRequest creates a new RetrieveOrderRequest.
func (c *Client) NewRetrieveOrderRequest() *RetrieveOrderRequest {
	return &RetrieveOrderRequest{client: c}
}

// RetrieveOrderResponse represents the response from Bitfinex order retrieval.
type RetrieveOrderResponse struct {
	Orders []Order
}

// UnmarshalJSON parses the Bitfinex array response into RetrieveOrderResponse.
func (r *RetrieveOrderResponse) UnmarshalJSON(data []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}

	for _, raw := range raws {
		var order Order

		if err := parseJsonArray(raw, &order, 0); err != nil {
			return err
		}

		r.Orders = append(r.Orders, order)
	}

	return nil
}

package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

// CloseFundingRequest represents a Bitfinex funding close request.
// API: https://docs.bitfinex.com/reference/rest-auth-funding-close
//
//go:generate requestgen -type CloseFundingRequest -method POST -url "/v2/auth/w/funding/close" -responseType .CloseFundingResponse
type CloseFundingRequest struct {
	client requestgen.AuthenticatedAPIClient

	// id is the Offer ID (retrievable via the Funding Loans and Funding Credits endpoints)
	id int64 `param:"id,required"` // The ID of the funding to close.
}

// NewCloseFundingRequest creates a new CloseFundingRequest.
func (c *FundingService) NewCloseFundingRequest() *CloseFundingRequest {
	return &CloseFundingRequest{client: c}
}

// CloseFundingResponse represents the response for funding close.
type CloseFundingResponse struct {
	Time   types.MillisecondTimestamp
	Type   string
	_      any
	_      any
	_      any
	_      any
	Status string
	_      any
}

// UnmarshalJSON parses the JSON array into the CloseFundingResponse struct fields.
func (r *CloseFundingResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

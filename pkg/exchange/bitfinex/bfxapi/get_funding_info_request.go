package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// GetFundingInfoRequest represents a Bitfinex funding info request.
// API: https://docs.bitfinex.com/reference/rest-auth-info-funding
//
//go:generate requestgen -type GetFundingInfoRequest -method POST -url "/v2/auth/r/info/funding/:key" -responseType .FundingInfoResponse
type GetFundingInfoRequest struct {
	client requestgen.AuthenticatedAPIClient

	key string `param:"key,slug"` // The funding key for which to retrieve info, e.g., "fUSD" for USD funding info.
}

// NewGetFundingInfoRequest creates a new GetFundingInfoRequest.
func (c *Client) NewGetFundingInfoRequest() *GetFundingInfoRequest {
	return &GetFundingInfoRequest{client: c}
}

// FundingInfoResponse represents the response for funding info.
type FundingInfoResponse struct {
	Type    string
	Symbol  string
	Details FundingInfoDetails
}

type FundingInfoDetails struct {
	YieldLoan    fixedpoint.Value
	YieldLend    fixedpoint.Value
	DurationLoan float64
	DurationLend float64
}

// UnmarshalJSON parses the JSON array into the FundingInfoResponse struct fields.
func (r *FundingInfoResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

// UnmarshalJSON parses the JSON array into the FundingInfoDetails struct fields.
func (d *FundingInfoDetails) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, d, 0)
}

package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// AutoRenewFundingRequest represents a Bitfinex funding auto-renew request.
// API: https://docs.bitfinex.com/reference/rest-auth-funding-auto-renew
//
// Body Params:
//   - status (int32, required): 1 to activate, 0 to deactivate
//   - currency (string, required): currency for which to enable auto-renew
//   - amount (string, optional): amount to be auto-renewed
//   - rate (string, optional): percentage rate at which to auto-renew
//   - period (int32, optional): period in days
//
//go:generate requestgen -type AutoRenewFundingRequest -method POST -url "/v2/auth/w/funding/auto" -responseType .AutoRenewFundingResponse
type AutoRenewFundingRequest struct {
	client requestgen.AuthenticatedAPIClient

	// status - 1 to activate, 0 to deactivate
	status int32 `param:"status,required" json:"status"`

	// currency is required, Defaults to USD
	currency string `param:"currency,required" json:"currency"`

	// amount is the amount to be auto-renewed (Minimum 50 USD equivalent). Defaultst to the amount currently provided if omitted.
	amount *string `param:"amount,omitempty" json:"amount,omitempty"`

	// rate is the percentage rate at which to auto-renew. (rate == 0 to renew at FRR). Defaults to FRR if omitted
	rate *string `param:"rate,omitempty" json:"rate,omitempty"`

	// period Defaults to 2
	period *int `param:"period,omitempty" json:"period,omitempty"`
}

// NewAutoRenewFundingRequest creates a new AutoRenewFundingRequest.
func (c *FundingService) NewAutoRenewFundingRequest() *AutoRenewFundingRequest {
	return &AutoRenewFundingRequest{client: c}
}

// AutoRenewFundingResponse represents the response for funding auto-renew.
type AutoRenewFundingResponse struct {
	Time        types.MillisecondTimestamp
	Type        string
	MessageID   any
	Placeholder any
	Offer       *RenewedFundingOffer
	Code        any
	Status      string
	Text        string
}

// UnmarshalJSON parses the JSON array into the AutoRenewFundingResponse struct fields.
func (r *AutoRenewFundingResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

// FundingOfferArray represents the funding offer array in the response.
type RenewedFundingOffer struct {
	Currency string

	// Period is the period in days for the funding offer.
	Period int

	// Rate is the percentage rate for the funding offer.
	// Rate of the offer (percentage expressed as decimal number i.e. 1% = 0.01)
	Rate fixedpoint.Value

	// Threshold is the max amount to be auto-renewed
	Threshold int
}

// UnmarshalJSON parses the JSON array into the FundingOfferArray struct fields.
func (o *RenewedFundingOffer) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, o, 0)
}

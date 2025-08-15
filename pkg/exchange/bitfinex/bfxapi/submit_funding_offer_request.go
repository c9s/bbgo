package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

// SubmitFundingOfferRequest represents a request to submit a funding offer to Bitfinex.
// API: https://docs.bitfinex.com/reference/rest-auth-submit-funding-offer
//
//go:generate requestgen -type SubmitFundingOfferRequest -method POST -url "/v2/auth/w/funding/offer/submit" -responseType .SubmitFundingOfferResponse
type SubmitFundingOfferRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol    string           `param:"symbol,required"` // e.g. fUSD
	amount    string           `param:"amount,required"` // offer amount
	rate      string           `param:"rate,required"`   // daily rate
	period    int              `param:"period,required"` // period in days
	offerType FundingOfferType `param:"type,required"`   // e.g. LIMIT
	flags     *int64           `param:"flags,omitempty"`
	autoRenew *bool            `param:"renew,omitempty"`
	hidden    *bool            `param:"hidden,omitempty"`
	notify    *bool            `param:"notify,omitempty"`
}

// NewSubmitFundingOfferRequest creates a new SubmitFundingOfferRequest instance.
func (c *FundingService) NewSubmitFundingOfferRequest() *SubmitFundingOfferRequest {
	return &SubmitFundingOfferRequest{client: c}
}

// SubmitFundingOfferResponse represents the response for submitting a funding offer.
type SubmitFundingOfferResponse struct {
	Mts          types.MillisecondTimestamp // response timestamp
	Type         string                     // response type
	MessageID    *int64                     // message ID
	_            any                        // placeholder
	FundingOffer FundingOffer               // funding offer details
	Code         *int64                     // response code
	Status       string                     // response status
	Text         string                     // response text
}

// UnmarshalJSON parses the Bitfinex array response into SubmitFundingOfferResponse.
func (r *SubmitFundingOfferResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

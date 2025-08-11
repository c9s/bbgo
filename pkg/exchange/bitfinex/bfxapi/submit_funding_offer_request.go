package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/boolint"
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
func (c *Client) NewSubmitFundingOfferRequest() *SubmitFundingOfferRequest {
	return &SubmitFundingOfferRequest{client: c}
}

// FundingOffer represents the funding offer details in the response.
type FundingOffer struct {
	ID          int64                      // offer ID
	Symbol      string                     // funding currency symbol
	CreatedAt   types.MillisecondTimestamp // creation timestamp
	UpdatedAt   types.MillisecondTimestamp // update timestamp
	Amount      fixedpoint.Value           // amount
	AmountOrig  fixedpoint.Value           // original amount
	OfferType   FundingOfferType           // offer type
	_           any
	_           any
	Flags       *int64             // flags
	OfferStatus FundingOfferStatus // offer status
	_           any
	_           any
	_           any
	Rate        fixedpoint.Value // rate
	Period      fixedpoint.Value // period in days
	Notify      *boolint.Value   // notify
	Hidden      *boolint.Value   // hidden
	_           any
	Renew       boolint.Value // auto renew
	Extra       any
}

// UnmarshalJSON parses the funding offer array into FundingOffer struct.
func (f *FundingOffer) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, f, 0)
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

// FundingOfferType represents the type of funding order for Bitfinex API.
type FundingOfferType string

const (
	FundingOfferTypeLimit       FundingOfferType = "LIMIT"       // Place an order at an explicit, static rate
	FundingOfferTypeFRRDeltaFix FundingOfferType = "FRRDELTAFIX" // Place an order at an implicit, static rate, relative to the FRR
	FundingOfferTypeFRRDeltaVar FundingOfferType = "FRRDELTAVAR" // Place an order at an implicit, dynamic rate, relative to the FRR
)

// String returns the string representation of the FundingOfferType.
func (t FundingOfferType) String() string {
	return string(t)
}

// FundingOfferStatus represents the status of a funding offer.
type FundingOfferStatus string

const (
	FundingOfferStatusActive          FundingOfferStatus = "ACTIVE"
	FundingOfferStatusPartiallyFilled FundingOfferStatus = "PARTIALLY FILLED"
)

// String returns the string representation of the FundingOfferStatus.
func (s FundingOfferStatus) String() string {
	return string(s)
}

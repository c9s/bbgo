package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

// CancelFundingOfferRequest represents a request to cancel a funding offer.
// API: https://docs.bitfinex.com/reference/rest-auth-cancel-funding-offer
//
//go:generate requestgen -type CancelFundingOfferRequest -method POST -url "/v2/auth/w/funding/offer/cancel" -responseType .CancelFundingOfferResponse
type CancelFundingOfferRequest struct {
	client requestgen.AuthenticatedAPIClient

	id int64 `param:"id,required"` // funding offer ID to cancel
}

// NewCancelFundingOfferRequest creates a new CancelFundingOfferRequest instance.
func (c *FundingService) NewCancelFundingOfferRequest() *CancelFundingOfferRequest {
	return &CancelFundingOfferRequest{client: c}
}

// CancelFundingOfferResponse represents the response for canceling a funding offer.
type CancelFundingOfferResponse struct {
	Mts          types.MillisecondTimestamp // response timestamp
	Type         string                     // response type
	MessageID    *int64                     // message ID
	_            any                        // placeholder
	FundingOffer FundingOffer               // funding offer details
	Code         *int64                     // response code
	Status       string                     // response status
	Text         *string                    // response text (nullable)
}

// UnmarshalJSON parses the Bitfinex array response into CancelFundingOfferResponse.
func (r *CancelFundingOfferResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

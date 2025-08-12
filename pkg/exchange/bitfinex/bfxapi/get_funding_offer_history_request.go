// Package bfxapi implements Bitfinex API integration.
package bfxapi

import (
	"github.com/c9s/requestgen"
)

// GetFundingOfferHistoryRequest represents a Bitfinex funding offer history request.
// API: https://docs.bitfinex.com/reference/rest-auth-funding-offers-hist
//
//go:generate requestgen -type GetFundingOfferHistoryRequest -method POST -url "/v2/auth/r/funding/offers/hist" -responseType []FundingOffer
type GetFundingOfferHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient
}

// NewGetFundingOfferHistoryRequest creates a new GetFundingOfferHistoryRequest.
func (c *Client) NewGetFundingOfferHistoryRequest() *GetFundingOfferHistoryRequest {
	return &GetFundingOfferHistoryRequest{client: c}
}

// FundingOffer represents a single funding offer returned by Bitfinex.
// Please reuse the existing FundingOffer struct defined elsewhere in the package.
// The response type for this request is []FundingOffer.

package bfxapi

import (
	"github.com/c9s/requestgen"
)

// GetActiveFundingOffersRequest represents a request for active funding offers.
// API: https://docs.bitfinex.com/reference/rest-auth-funding-offers
//
//go:generate requestgen -type GetActiveFundingOffersRequest -method POST -url "/v2/auth/r/funding/offers/:symbol" -responseType []FundingOffer
type GetActiveFundingOffersRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol,slug"` // Trading pair symbol (e.g., "fUSD")
}

// NewGetActiveFundingOffersRequest creates a new GetActiveFundingOffersRequest instance.
func (c *FundingService) NewGetActiveFundingOffersRequest() *GetActiveFundingOffersRequest {
	return &GetActiveFundingOffersRequest{client: c}
}

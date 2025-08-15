package bfxapi

import (
	"github.com/c9s/requestgen"
)

// GetFundingTradeHistoryRequest represents a Bitfinex funding trade history request.
// API: https://docs.bitfinex.com/reference/rest-auth-funding-trades-hist
//
//go:generate requestgen -type GetFundingTradeHistoryRequest -method POST -url "/v2/auth/r/funding/trades/:symbol/hist" -responseType []FundingTrade
type GetFundingTradeHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol,slug"` // The symbol for which to retrieve funding trade history, e.g., "fUSD" for USD funding trades.
}

// NewGetFundingTradeHistoryRequest creates a new GetFundingTradeHistoryRequest.
func (c *FundingService) NewGetFundingTradeHistoryRequest() *GetFundingTradeHistoryRequest {
	return &GetFundingTradeHistoryRequest{client: c}
}

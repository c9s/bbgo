package bfxapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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
func (c *Client) NewGetFundingTradeHistoryRequest() *GetFundingTradeHistoryRequest {
	return &GetFundingTradeHistoryRequest{client: c}
}

// FundingTrade represents a single funding trade returned by Bitfinex.
type FundingTrade struct {
	ID        int64
	Currency  string
	CreatedAt types.MillisecondTimestamp
	OfferID   int64
	Amount    fixedpoint.Value
	Rate      fixedpoint.Value
	Period    int
	// Placeholder field for the last element, nullable
	Placeholder any
}

// UnmarshalJSON parses the JSON array into the FundingTrade struct fields.
func (t *FundingTrade) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, t, 0)
}

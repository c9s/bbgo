package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// GetSummaryRequest represents a Bitfinex summary request.
// API: https://docs.bitfinex.com/reference/rest-auth-summary
//
//go:generate requestgen -type GetSummaryRequest -method POST -url "/v2/auth/r/summary" -responseType .SummaryResponse
type GetSummaryRequest struct {
	client requestgen.AuthenticatedAPIClient
}

// NewGetSummaryRequest creates a new GetSummaryRequest.
func (c *Client) NewGetSummaryRequest() *GetSummaryRequest {
	return &GetSummaryRequest{client: c}
}

// SummaryResponse represents the response for summary API.
type SummaryResponse struct {
	_ any
	_ any
	_ any
	_ any

	// Array with info on your current fee rates
	FeeInfo *FeeInfoArray

	// Array with data on your trading volume and fees paid
	TradingVolAndFee *TradingVolAndFee

	// Array with data on your funding earnings
	FundingEarnings *FundingEarnings
	_               any
	_               any

	// LeoInfo: Object with info on your LEO level and holdings. Keys: "leo_lev" (to see your current LEO level) and "leo_amount_avg" (to see your average LEO amount held in the past 30 days)
	LeoInfo *LeoInfo
}

// UnmarshalJSON parses the JSON array into the SummaryResponse struct fields.
func (r *SummaryResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

// FeeInfoArray contains maker/taker fee info.
type FeeInfoArray struct {
	MakerFeeInfo FeeInfo
	TakerFeeInfo FeeInfo
}

// UnmarshalJSON parses the JSON array into the FeeInfoArray struct fields.
func (f *FeeInfoArray) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, f, 0)
}

type FeeInfo struct {
	CryptoFee   fixedpoint.Value
	StableFee   fixedpoint.Value
	FiatFee     fixedpoint.Value
	_           any
	_           any
	DerivRebate fixedpoint.Value
}

func (f *FeeInfo) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, f, 0)
}

// TradingVolAndFee contains trading volume and fee info.
type TradingVolAndFee struct {
	TradeVol30d         []TradeVol30dEntry
	FeesTrading30d      map[string]fixedpoint.Value
	FeesTradingTotal30d fixedpoint.Value
}

// UnmarshalJSON parses the JSON array into the TradingVolAndFee struct fields.
func (t *TradingVolAndFee) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, t, 0)
}

type TradeVol30dEntry struct {
	Curr        string            `json:"curr"`
	Vol         fixedpoint.Value  `json:"vol"`
	VolSafe     *fixedpoint.Value `json:"vol_safe,omitempty"`
	VolMaker    *fixedpoint.Value `json:"vol_maker,omitempty"`
	VolBFX      *fixedpoint.Value `json:"vol_BFX,omitempty"`
	VolBFXSafe  *fixedpoint.Value `json:"vol_BFX_safe,omitempty"`
	VolBFXMaker *fixedpoint.Value `json:"vol_BFX_maker,omitempty"`
}

// FundingEarnings contains funding earnings info.
type FundingEarnings struct {
	Placeholder1           any
	FundingEarningsPerCurr map[string]fixedpoint.Value
	FundingEarningsTotal   fixedpoint.Value
}

func (f *FundingEarnings) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, f, 0)
}

// LeoInfo contains LEO token info.
type LeoInfo struct {
	LeoLev       fixedpoint.Value `json:"leo_lev"`
	LeoAmountAvg fixedpoint.Value `json:"leo_amount_avg"`
}

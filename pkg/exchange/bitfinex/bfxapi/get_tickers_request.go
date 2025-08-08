package bfxapi

import (
	"encoding/json"
	"strconv"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// FundingTicker for funding tickers (symbol starts with 'f')
type FundingTicker struct {
	Symbol              string
	FRR                 fixedpoint.Value
	Bid                 fixedpoint.Value
	BidPeriod           fixedpoint.Value
	BidSize             fixedpoint.Value
	Ask                 fixedpoint.Value
	AskPeriod           fixedpoint.Value
	AskSize             fixedpoint.Value
	DailyChange         fixedpoint.Value
	DailyChangeRelative fixedpoint.Value
	LastPrice           fixedpoint.Value
	Volume              fixedpoint.Value
	High                fixedpoint.Value
	Low                 fixedpoint.Value
	P1                  any
	P2                  any
	FRRAmountAvailable  *fixedpoint.Value // nullable
}

// UnmarshalJSON parses a futures ticker response from a JSON array.
func (r *FundingTicker) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

// TickersResponse is a slice of interface{} (Ticker or FundingTicker)
type TickersResponse struct {
	FundingTickers []FundingTicker
	TradingTickers []Ticker
}

// UnmarshalJSON parses the /v2/tickers response.
func (r *TickersResponse) UnmarshalJSON(data []byte) error {
	var arr [][]json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}

	for _, entry := range arr {
		symbol, _ := strconv.Unquote(string(entry[0]))
		switch symbol[0] {
		case 't':
			var ticker Ticker
			if err := parseRawArray(entry, &ticker, 0); err != nil {
				return err
			}

			r.TradingTickers = append(r.TradingTickers, ticker)

		case 'f':
			var ticker FundingTicker
			if err := parseRawArray(entry, &ticker, 0); err != nil {
				return err
			}

			r.FundingTickers = append(r.FundingTickers, ticker)
		}

	}

	return nil
}

//go:generate requestgen -type GetTickersRequest -method GET -url "/v2/tickers" -responseType .TickersResponse
type GetTickersRequest struct {
	client  requestgen.APIClient
	symbols string `param:"symbols,query"` // comma separated symbols, e.g. tBTCUSD,fUSD
}

// NewGetTickersRequest creates a new GetTickersRequest.
func (c *Client) NewGetTickersRequest() *GetTickersRequest {
	return &GetTickersRequest{client: c}
}

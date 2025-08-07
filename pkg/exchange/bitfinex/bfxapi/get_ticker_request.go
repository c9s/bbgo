package bfxapi

import (
	"encoding/json"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

/*
Ticker JSON response:

[
  10645, // BID
  73.93854271, // BID_SIZE
  10647, // ASK
  75.22266119, // ASK_SIZE
  731.60645389, // DAILY_CHANGE
  0.0738, // DAILY_CHANGE_RELATIVE
  10644.00645389, // LAST_PRICE
  14480.89849423, // VOLUME
  10766, // HIGH
  9889.1449809 // LOW
]
*/

type TickerResponse struct {
	Bid     fixedpoint.Value
	BidSize fixedpoint.Value

	Ask     fixedpoint.Value
	AskSize fixedpoint.Value

	DailyChange         fixedpoint.Value
	DailyChangeRelative fixedpoint.Value
	LastPrice           fixedpoint.Value

	Volume fixedpoint.Value
	High   fixedpoint.Value
	Low    fixedpoint.Value
}

func (r *TickerResponse) UnmarshalJSON(data []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}

	return parseArray(raws, r)
}

// API: https://api-pub.bitfinex.com/v2/ticker/{symbol}
//
//go:generate requestgen -type GetTickerRequest -method GET -url "/v2/ticker/:symbol" -responseType .TickerResponse
type GetTickerRequest struct {
	client requestgen.APIClient

	symbol string `param:"symbol,slug"` // e.g. tBTCUSD, tETHUSD, etc.
}

func (c *Client) NewGetTickerRequest() *GetTickerRequest {
	return &GetTickerRequest{client: c}
}

package bfxapi

import (
	"github.com/c9s/requestgen"
)

// GetCandlesRequest represents a request for Bitfinex public candles endpoint.
//
//go:generate requestgen -type GetCandlesRequest -method GET -url "/v2/candles/:candle/:section" -responseType .CandlesResponse
type GetCandlesRequest struct {
	client requestgen.APIClient

	candle string `param:"candle,slug"` // e.g. tBTCUSD

	section string `param:"section,slug" validValues:"last,hist"` // e.g. last, hist

	// sort: +1: sort in ascending order | -1: sort in descending order (by MTS field).
	sort int `param:"sort"` // optional, 1 for ascending, -1 for descending order

	// start: If start is given, only records with MTS >= start (milliseconds) will be given as response.
	start int64 `param:"start"` // optional, millisecond timestamp

	// end: If end is given, only records with MTS <= end (milliseconds) will be given as response.
	end int64 `param:"end"` // optional, millisecond timestamp

	// limit: Number of records in response (max. 10000).
	limit int `param:"limit"` // optional, max number of candles
}

// NewGetCandlesRequest creates a new GetCandlesRequest.
func (c *Client) NewGetCandlesRequest() *GetCandlesRequest {
	return &GetCandlesRequest{client: c}
}

// CandlesResponse represents multiple candles response.
type CandlesResponse []Candle

// CandleTimeFrames defines available Bitfinex candle time frames.
//
//go:generate mapgen -type TimeFrame
type TimeFrame string

const (
	TimeFrame1m  TimeFrame = "1m"
	TimeFrame5m  TimeFrame = "5m"
	TimeFrame15m TimeFrame = "15m"
	TimeFrame30m TimeFrame = "30m"
	TimeFrame1h  TimeFrame = "1h"
	TimeFrame3h  TimeFrame = "3h"
	TimeFrame6h  TimeFrame = "6h"
	TimeFrame12h TimeFrame = "12h"
	TimeFrame1D  TimeFrame = "1D"
	TimeFrame1W  TimeFrame = "1W"
	TimeFrame14D TimeFrame = "14D"
	TimeFrame1M  TimeFrame = "1M"
)

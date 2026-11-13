package backpackapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types/strint"
)

//go:generate -command GetRequest requestgen -method GET

type Kline struct {
	// Start and End are UTC datetimes without a timezone suffix, e.g. "2026-09-01 17:23:00".
	Start KlineTime `json:"start"`
	End   KlineTime `json:"end"`

	// Open, High, Low and Close are null for intervals with no trades.
	Open  fixedpoint.Value `json:"open"`
	High  fixedpoint.Value `json:"high"`
	Low   fixedpoint.Value `json:"low"`
	Close fixedpoint.Value `json:"close"`

	Volume      fixedpoint.Value `json:"volume"`
	QuoteVolume fixedpoint.Value `json:"quoteVolume"`

	Trades strint.Int64 `json:"trades"`
}

// GetKlinesRequest queries the candlesticks of a market.
//
// Note that unlike the history endpoints, startTime and endTime are unix timestamps in
// **seconds**.
//
//go:generate GetRequest -url "/api/v1/klines" -type GetKlinesRequest -responseType []Kline
type GetKlinesRequest struct {
	client requestgen.APIClient

	symbol   string        `param:"symbol,required"`
	interval KlineInterval `param:"interval,required"`

	startTime time.Time `param:"startTime,required,seconds"`

	// endTime defaults to the current time when omitted.
	endTime *time.Time `param:"endTime,seconds"`

	priceType *KlinePriceType `param:"priceType"`
	source    *KlineSource    `param:"source"`
}

func (c *RestClient) NewGetKlinesRequest() *GetKlinesRequest {
	return &GetKlinesRequest{client: c}
}

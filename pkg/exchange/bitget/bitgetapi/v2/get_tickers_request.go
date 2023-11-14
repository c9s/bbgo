package bitgetapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type Ticker struct {
	Symbol       string                     `json:"symbol"`
	High24H      fixedpoint.Value           `json:"high24h"`
	Open         fixedpoint.Value           `json:"open"`
	Low24H       fixedpoint.Value           `json:"low24h"`
	LastPr       fixedpoint.Value           `json:"lastPr"`
	QuoteVolume  fixedpoint.Value           `json:"quoteVolume"`
	BaseVolume   fixedpoint.Value           `json:"baseVolume"`
	UsdtVolume   fixedpoint.Value           `json:"usdtVolume"`
	BidPr        fixedpoint.Value           `json:"bidPr"`
	AskPr        fixedpoint.Value           `json:"askPr"`
	BidSz        fixedpoint.Value           `json:"bidSz"`
	AskSz        fixedpoint.Value           `json:"askSz"`
	OpenUtc      fixedpoint.Value           `json:"openUtc"`
	Ts           types.MillisecondTimestamp `json:"ts"`
	ChangeUtc24H fixedpoint.Value           `json:"changeUtc24h"`
	Change24H    fixedpoint.Value           `json:"change24h"`
}

//go:generate GetRequest -url "/api/v2/spot/market/tickers" -type GetTickersRequest -responseDataType []Ticker
type GetTickersRequest struct {
	client requestgen.APIClient

	symbol *string `param:"symbol,query"`
}

func (s *Client) NewGetTickersRequest() *GetTickersRequest {
	return &GetTickersRequest{client: s.Client}
}

package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Ticker struct {
	Symbol    string                     `json:"symbol"`
	High24H   fixedpoint.Value           `json:"high24h"`
	Low24H    fixedpoint.Value           `json:"low24h"`
	Close     fixedpoint.Value           `json:"close"`
	QuoteVol  fixedpoint.Value           `json:"quoteVol"`
	BaseVol   fixedpoint.Value           `json:"baseVol"`
	UsdtVol   fixedpoint.Value           `json:"usdtVol"`
	Ts        types.MillisecondTimestamp `json:"ts"`
	BuyOne    fixedpoint.Value           `json:"buyOne"`
	SellOne   fixedpoint.Value           `json:"sellOne"`
	BidSz     fixedpoint.Value           `json:"bidSz"`
	AskSz     fixedpoint.Value           `json:"askSz"`
	OpenUtc0  fixedpoint.Value           `json:"openUtc0"`
	ChangeUtc fixedpoint.Value           `json:"changeUtc"`
	Change    fixedpoint.Value           `json:"change"`
}

//go:generate GetRequest -url "/api/spot/v1/market/ticker" -type GetTickerRequest -responseDataType .Ticker
type GetTickerRequest struct {
	client requestgen.APIClient

	symbol string `param:"symbol"`
}

func (c *RestClient) NewGetTickerRequest() *GetTickerRequest {
	return &GetTickerRequest{client: c}
}

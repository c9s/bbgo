package backpackapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types/strint"
)

//go:generate -command GetRequest requestgen -method GET

type Ticker struct {
	Symbol string `json:"symbol"`

	FirstPrice fixedpoint.Value `json:"firstPrice"`
	LastPrice  fixedpoint.Value `json:"lastPrice"`
	High       fixedpoint.Value `json:"high"`
	Low        fixedpoint.Value `json:"low"`

	PriceChange        fixedpoint.Value `json:"priceChange"`
	PriceChangePercent fixedpoint.Value `json:"priceChangePercent"`

	Volume      fixedpoint.Value `json:"volume"`
	QuoteVolume fixedpoint.Value `json:"quoteVolume"`

	// Trades is the number of trades in the interval, sent as a JSON string.
	Trades strint.Int64 `json:"trades"`
}

//go:generate GetRequest -url "/api/v1/ticker" -type GetTickerRequest -responseType .Ticker
type GetTickerRequest struct {
	client requestgen.APIClient

	symbol string `param:"symbol,required"`

	interval *TickerInterval `param:"interval"`
	source   *KlineSource    `param:"source"`
}

func (c *RestClient) NewGetTickerRequest() *GetTickerRequest {
	return &GetTickerRequest{client: c}
}

//go:generate GetRequest -url "/api/v1/tickers" -type GetTickersRequest -responseType []Ticker
type GetTickersRequest struct {
	client requestgen.APIClient

	interval *TickerInterval `param:"interval"`
	source   *KlineSource    `param:"source"`
}

func (c *RestClient) NewGetTickersRequest() *GetTickersRequest {
	return &GetTickersRequest{client: c}
}

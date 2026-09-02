package backpackapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET

type Trade struct {
	Id int64 `json:"id"`

	Price         fixedpoint.Value `json:"price"`
	Quantity      fixedpoint.Value `json:"quantity"`
	QuoteQuantity fixedpoint.Value `json:"quoteQuantity"`

	Timestamp types.MillisecondTimestamp `json:"timestamp"`

	IsBuyerMaker bool `json:"isBuyerMaker"`
}

//go:generate GetRequest -url "/api/v1/trades" -type GetTradesRequest -responseType []Trade
type GetTradesRequest struct {
	client requestgen.APIClient

	symbol string `param:"symbol,required"`

	// limit defaults to 100 and is capped at 1000.
	limit *uint64 `param:"limit"`
}

func (c *RestClient) NewGetTradesRequest() *GetTradesRequest {
	return &GetTradesRequest{client: c}
}

// GetTradesHistoryRequest queries the historical public trades of a market.
//
// The API requires offset + limit to be at most 10000.
//
//go:generate GetRequest -url "/api/v1/trades/history" -type GetTradesHistoryRequest -responseType []Trade -rateLimiter 1+30/60s
type GetTradesHistoryRequest struct {
	client requestgen.APIClient

	symbol string `param:"symbol,required"`

	limit  *uint64 `param:"limit"`
	offset *uint64 `param:"offset"`
}

func (c *RestClient) NewGetTradesHistoryRequest() *GetTradesHistoryRequest {
	return &GetTradesHistoryRequest{client: c}
}

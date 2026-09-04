package backpackapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET

type MarkPrice struct {
	Symbol string `json:"symbol"`

	MarkPrice   fixedpoint.Value `json:"markPrice"`
	IndexPrice  fixedpoint.Value `json:"indexPrice"`
	FundingRate fixedpoint.Value `json:"fundingRate"`

	NextFundingTimestamp types.MillisecondTimestamp `json:"nextFundingTimestamp"`
}

//go:generate GetRequest -url "/api/v1/markPrices" -type GetMarkPricesRequest -responseType []MarkPrice
type GetMarkPricesRequest struct {
	client requestgen.APIClient

	symbol *string `param:"symbol"`

	// marketType defaults to the perp markets when omitted.
	marketType *MarketType `param:"marketType"`
}

func (c *RestClient) NewGetMarkPricesRequest() *GetMarkPricesRequest {
	return &GetMarkPricesRequest{client: c}
}

type OpenInterest struct {
	Symbol       string                     `json:"symbol"`
	OpenInterest fixedpoint.Value           `json:"openInterest"`
	Timestamp    types.MillisecondTimestamp `json:"timestamp"`
}

//go:generate GetRequest -url "/api/v1/openInterest" -type GetOpenInterestRequest -responseType []OpenInterest
type GetOpenInterestRequest struct {
	client requestgen.APIClient

	symbol *string `param:"symbol"`
}

func (c *RestClient) NewGetOpenInterestRequest() *GetOpenInterestRequest {
	return &GetOpenInterestRequest{client: c}
}

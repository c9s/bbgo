package ftxapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result
//go:generate -command DeleteRequest requestgen -method DELETE -responseType .APIResponse -responseDataField Result

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Market struct {
	Name                  string           `json:"name"`
	BaseCurrency          string           `json:"baseCurrency"`
	QuoteCurrency         string           `json:"quoteCurrency"`
	QuoteVolume24H        fixedpoint.Value `json:"quoteVolume24h"`
	Change1H              fixedpoint.Value `json:"change1h"`
	Change24H             fixedpoint.Value `json:"change24h"`
	ChangeBod             fixedpoint.Value `json:"changeBod"`
	VolumeUsd24H          fixedpoint.Value `json:"volumeUsd24h"`
	HighLeverageFeeExempt bool             `json:"highLeverageFeeExempt"`
	MinProvideSize        fixedpoint.Value `json:"minProvideSize"`
	Type                  string           `json:"type"`
	Underlying            string           `json:"underlying"`
	Enabled               bool             `json:"enabled"`
	Ask                   fixedpoint.Value `json:"ask"`
	Bid                   fixedpoint.Value `json:"bid"`
	Last                  fixedpoint.Value `json:"last"`
	PostOnly              bool             `json:"postOnly"`
	Price                 fixedpoint.Value `json:"price"`
	PriceIncrement        fixedpoint.Value `json:"priceIncrement"`
	SizeIncrement         fixedpoint.Value `json:"sizeIncrement"`
	Restricted            bool             `json:"restricted"`
}

//go:generate GetRequest -url "api/markets" -type GetMarketsRequest -responseDataType []Market
type GetMarketsRequest struct {
	client requestgen.APIClient
}

func (c *RestClient) NewGetMarketsRequest() *GetMarketsRequest {
	return &GetMarketsRequest{
		client: c,
	}
}

//go:generate GetRequest -url "api/markets/:market" -type GetMarketRequest -responseDataType .Market
type GetMarketRequest struct {
	client requestgen.AuthenticatedAPIClient
	market string `param:"market,slug"`
}

func (c *RestClient) NewGetMarketRequest(market string) *GetMarketRequest {
	return &GetMarketRequest{
		client: c,
		market: market,
	}
}

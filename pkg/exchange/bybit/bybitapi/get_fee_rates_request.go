package bybitapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type FeeRates struct {
	List []FeeRate `json:"list"`
}

type FeeRate struct {
	Symbol       string           `json:"symbol"`
	TakerFeeRate fixedpoint.Value `json:"takerFeeRate"`
	MakerFeeRate fixedpoint.Value `json:"makerFeeRate"`
}

//go:generate GetRequest -url "/v5/account/fee-rate" -type GetFeeRatesRequest -responseDataType .FeeRates
type GetFeeRatesRequest struct {
	client requestgen.AuthenticatedAPIClient

	category Category `param:"category,query" validValues:"spot"`
	// Symbol name. Valid for linear, inverse, spot
	symbol *string `param:"symbol,query"`
	// Base coin. SOL, BTC, ETH. Valid for option
	baseCoin *string `param:"baseCoin,query"`
}

func (c *RestClient) NewGetFeeRatesRequest() *GetFeeRatesRequest {
	return &GetFeeRatesRequest{
		client:   c,
		category: CategorySpot,
	}
}

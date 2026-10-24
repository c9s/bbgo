package amberdataapi

import (
	"time"

	"github.com/c9s/requestgen"
)

type FundingRatesResponse = Response[FundingRate]

//go:generate requestgen -method GET -url "/markets/:assetClass/funding-rates/:instrument" -type GetFundingRatesRequest -responseType .FundingRatesResponse
type GetFundingRatesRequest struct {
	client requestgen.AuthenticatedAPIClient

	assetClass AssetClass `param:"assetClass,slug,required"`
	instrument string     `param:"instrument,slug,required"`

	exchange string `param:"exchange,query,required"`

	startDate *time.Time `param:"startDate,query,milliseconds"`
	endDate   *time.Time `param:"endDate,query,milliseconds"`

	timeFormat string `param:"timeFormat,query,required"`
}

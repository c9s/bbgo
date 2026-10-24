package amberdataapi

import (
	"time"

	"github.com/c9s/requestgen"
)

type OpenInterestResponse = Response[OpenInterest]

//go:generate requestgen -method GET -url "/markets/:assetClass/open-interest/:instrument" -type GetOpenInterestRequest -responseType .OpenInterestResponse
type GetOpenInterestRequest struct {
	client requestgen.AuthenticatedAPIClient

	assetClass AssetClass `param:"assetClass,slug,required"`
	instrument string     `param:"instrument,slug,required"`

	exchange string `param:"exchange,query,required"`

	startDate *time.Time `param:"startDate,query,milliseconds"`
	endDate   *time.Time `param:"endDate,query,milliseconds"`

	timeFormat string `param:"timeFormat,query,required"`
}

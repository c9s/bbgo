package amberdataapi

import (
	"time"

	"github.com/c9s/requestgen"
)

type LiquidationsResponse = Response[Liquidation]

//go:generate requestgen -method GET -url "/markets/:assetClass/liquidations/:instrument" -type GetLiquidationsRequest -responseType .LiquidationsResponse
type GetLiquidationsRequest struct {
	client requestgen.AuthenticatedAPIClient

	assetClass AssetClass `param:"assetClass,slug,required"`
	instrument string     `param:"instrument,slug,required"`

	exchange string `param:"exchange,query,required"`

	startDate *time.Time `param:"startDate,query,milliseconds"`
	endDate   *time.Time `param:"endDate,query,milliseconds"`

	timeFormat string `param:"timeFormat,query,required"`
}

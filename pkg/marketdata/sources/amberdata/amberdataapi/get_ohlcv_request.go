package amberdataapi

import (
	"time"

	"github.com/c9s/requestgen"
)

type OHLCVResponse = Response[OHLCV]

//go:generate requestgen -method GET -url "/markets/:assetClass/ohlcv/:instrument" -type GetOHLCVRequest -responseType .OHLCVResponse
type GetOHLCVRequest struct {
	client requestgen.AuthenticatedAPIClient

	assetClass AssetClass `param:"assetClass,slug,required"`
	instrument string     `param:"instrument,slug,required"`

	exchange string `param:"exchange,query,required"`

	startDate *time.Time `param:"startDate,query,milliseconds"`
	endDate   *time.Time `param:"endDate,query,milliseconds"`

	timeFormat string `param:"timeFormat,query,required"`

	// timeInterval selects the bucket: minutes, hours or days. There is no
	// finer or coarser option, so intervals below a minute or above a day have
	// no equivalent.
	timeInterval *string `param:"timeInterval,query"`
}

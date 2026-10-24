package amberdataapi

import (
	"time"

	"github.com/c9s/requestgen"
)

// TradesResponse is the envelope the trades endpoints return.
//
// requestgen needs a named type for -responseType, so each endpoint gets an
// alias of the generic envelope rather than the instantiation being written
// inline.
type TradesResponse = Response[Trade]

//go:generate requestgen -method GET -url "/markets/:assetClass/trades/:instrument" -type GetTradesRequest -responseType .TradesResponse
type GetTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	// assetClass is futures or spot. Both families expose the same endpoints
	// under a different prefix, so it is a slug rather than two request types.
	assetClass AssetClass `param:"assetClass,slug,required"`

	instrument string `param:"instrument,slug,required"`

	// exchange is required and only one is accepted per request.
	exchange string `param:"exchange,query,required"`

	// startDate is inclusive, endDate exclusive. Omitting both returns the
	// previous 24 hours.
	startDate *time.Time `param:"startDate,query,milliseconds"`
	endDate   *time.Time `param:"endDate,query,milliseconds"`

	// timeFormat is always sent as milliseconds; see NewGetTradesRequest.
	timeFormat string `param:"timeFormat,query,required"`
}

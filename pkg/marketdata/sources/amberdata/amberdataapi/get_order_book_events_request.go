package amberdataapi

import (
	"time"

	"github.com/c9s/requestgen"
)

// GetOrderBookEventsRequest fetches incremental book changes. Only changed
// levels appear, and a level with volume zero means it was removed.

//go:generate requestgen -method GET -url "/markets/:assetClass/order-book-events/:instrument" -type GetOrderBookEventsRequest -responseType .BookResponse
type GetOrderBookEventsRequest struct {
	client requestgen.AuthenticatedAPIClient

	assetClass AssetClass `param:"assetClass,slug,required"`
	instrument string     `param:"instrument,slug,required"`

	exchange string `param:"exchange,query,required"`

	startDate *time.Time `param:"startDate,query,milliseconds"`
	endDate   *time.Time `param:"endDate,query,milliseconds"`

	timeFormat string `param:"timeFormat,query,required"`

	maxLevel *int `param:"maxLevel,query"`
}

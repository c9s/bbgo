package amberdataapi

import (
	"time"

	"github.com/c9s/requestgen"
)

// BookResponse is the envelope both order book endpoints return. They share a
// shape, so nothing in the payload says which one was called: snapshot versus
// update is decided by the endpoint.
type BookResponse = Response[Book]

//go:generate requestgen -method GET -url "/markets/:assetClass/order-book-snapshots/:instrument" -type GetOrderBookSnapshotsRequest -responseType .BookResponse
type GetOrderBookSnapshotsRequest struct {
	client requestgen.AuthenticatedAPIClient

	assetClass AssetClass `param:"assetClass,slug,required"`
	instrument string     `param:"instrument,slug,required"`

	exchange string `param:"exchange,query,required"`

	startDate *time.Time `param:"startDate,query,milliseconds"`
	endDate   *time.Time `param:"endDate,query,milliseconds"`

	timeFormat string `param:"timeFormat,query,required"`

	// maxLevel caps the depth returned. Its exact semantics are unverified; see
	// the UNVERIFIED block in the parent package.
	maxLevel *int `param:"maxLevel,query"`
}

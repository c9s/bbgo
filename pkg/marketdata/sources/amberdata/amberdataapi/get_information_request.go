package amberdataapi

import "github.com/c9s/requestgen"

// CoverageResponse is the envelope the `*-information` endpoints return. Every
// data family has one, reporting the range each instrument actually has data
// for, which is how a backfill avoids requesting years that do not exist.
type CoverageResponse = Response[InstrumentCoverage]

// GetCoverageRequest fetches an instrument coverage listing.
//
// The feature is a slug — trades, order-book-snapshots, ohlcv and so on — so one
// request type serves every family's information endpoint rather than one per
// family.
//
//go:generate requestgen -method GET -url "/markets/:assetClass/:feature/information" -type GetCoverageRequest -responseType .CoverageResponse
type GetCoverageRequest struct {
	client requestgen.AuthenticatedAPIClient

	assetClass AssetClass `param:"assetClass,slug,required"`
	feature    string     `param:"feature,slug,required"`

	// exchange is optional here: omitting it lists every exchange covered.
	exchange *string `param:"exchange,query"`

	timeFormat string `param:"timeFormat,query,required"`
}

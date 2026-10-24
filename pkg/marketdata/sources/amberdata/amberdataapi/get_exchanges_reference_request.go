package amberdataapi

import "github.com/c9s/requestgen"

// ReferenceResponse is the envelope the exchanges/reference endpoints return.
type ReferenceResponse = Response[InstrumentReference]

// GetExchangesReferenceRequest fetches contract metadata.
//
// This is the endpoint that settles how to interpret a futures volume: it
// carries contractSize and underlyingToPositionMultiplier, and some venues
// report volume in contracts rather than in the base asset.
//
//go:generate requestgen -method GET -url "/markets/:assetClass/exchanges/reference" -type GetExchangesReferenceRequest -responseType .ReferenceResponse
type GetExchangesReferenceRequest struct {
	client requestgen.AuthenticatedAPIClient

	assetClass AssetClass `param:"assetClass,slug,required"`

	exchange *string `param:"exchange,query"`

	// instrument narrows the listing to one contract.
	instrument *string `param:"instrument,query"`

	timeFormat string `param:"timeFormat,query,required"`
}

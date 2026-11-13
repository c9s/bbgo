package backpackapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types/strint"
)

//go:generate -command GetRequest requestgen -method GET

type Depth struct {
	// Asks and Bids are both returned in ascending price order.
	Asks []PriceLevel `json:"asks"`
	Bids []PriceLevel `json:"bids"`

	// LastUpdateId is sent as a JSON string.
	LastUpdateId strint.Int64 `json:"lastUpdateId"`

	// Timestamp is the matching engine timestamp, in microseconds.
	Timestamp MicrosecondTimestamp `json:"timestamp"`
}

//go:generate GetRequest -url "/api/v1/depth" -type GetDepthRequest -responseType .Depth
type GetDepthRequest struct {
	client requestgen.APIClient

	symbol string `param:"symbol,required"`

	// limit is the number of levels to return. If omitted, up to 5000 levels are returned.
	limit *DepthLimit `param:"limit"`
}

func (c *RestClient) NewGetDepthRequest() *GetDepthRequest {
	return &GetDepthRequest{client: c}
}

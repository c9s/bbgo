package v3

//go:generate -command GetRequest requestgen -method GET

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/requestgen"
)

type Depth struct {
	Timestamp         int64                `json:"timestamp"`
	LastUpdateVersion int64                `json:"last_update_version"`
	LastUpdateId      int64                `json:"last_update_id"`
	Bids              [][]fixedpoint.Value `json:"bids"`
	Asks              [][]fixedpoint.Value `json:"asks"`
}

//go:generate GetRequest -url "/api/v3/depth" -type GetDepthRequest -responseType Depth
type GetDepthRequest struct {
	client requestgen.APIClient

	market      string `param:"market,required"`
	limit       *int   `param:"limit"`
	sortByPrice *bool  `param:"sort_by_price"`
}

func (s *Client) NewGetDepthRequest() *GetDepthRequest {
	return &GetDepthRequest{client: s.Client}
}

package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Depth struct {
	LastUpdateId int64                `json:"lastUpdateId"`
	Bids         [][]fixedpoint.Value `json:"bids"`
	Asks         [][]fixedpoint.Value `json:"asks"`
}

//go:generate requestgen -method GET -url "/api/v3/depth" -type GetDepthRequest -responseType .Depth
type GetDepthRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol"`
	limit  int    `param:"limit" defaultValue:"1000"`
}

func (c *RestClient) NewGetDepthRequest() *GetDepthRequest {
	return &GetDepthRequest{client: c}
}

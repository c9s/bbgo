package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Depth struct {
	Asks      [][]fixedpoint.Value       `json:"asks"`
	Bids      [][]fixedpoint.Value       `json:"bids"`
	Timestamp types.MillisecondTimestamp `json:"timestamp"`
}

//go:generate GetRequest -url "/api/spot/v1/market/depth" -type GetDepthRequest -responseDataType .Depth
type GetDepthRequest struct {
	client requestgen.APIClient

	symbol   string `param:"symbol"`
	stepType string `param:"type" default:"step0"`
	limit    *int   `param:"limit"`
}

func (c *RestClient) NewGetDepthRequest() *GetDepthRequest {
	return &GetDepthRequest{client: c}
}

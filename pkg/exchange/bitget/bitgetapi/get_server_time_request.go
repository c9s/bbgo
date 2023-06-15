package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

type ServerTime = types.MillisecondTimestamp

//go:generate GetRequest -url "/api/spot/v1/public/time" -type GetServerTimeRequest -responseDataType .ServerTime
type GetServerTimeRequest struct {
	client requestgen.APIClient
}

func (c *RestClient) NewGetServerTimeRequest() *GetServerTimeRequest {
	return &GetServerTimeRequest{client: c}
}

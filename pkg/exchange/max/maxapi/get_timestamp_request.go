package max

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

type Timestamp int64

//go:generate GetRequest -url "/api/v2/timestamp" -type GetTimestampRequest -responseType .Timestamp
type GetTimestampRequest struct {
	client requestgen.APIClient
}

func (c *RestClient) NewGetTimestampRequest() *GetTimestampRequest {
	return &GetTimestampRequest{client: c}
}

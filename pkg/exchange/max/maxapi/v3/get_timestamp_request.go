package v3

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

type TimestampResponse struct {
	TimestampInSeconds int64 `json:"timestamp"`
}

//go:generate GetRequest -url "/api/v3/timestamp" -type GetTimestampRequest -responseType .TimestampResponse
type GetTimestampRequest struct {
	client requestgen.APIClient
}

func (c *Client) NewGetTimestampRequest() *GetTimestampRequest {
	return &GetTimestampRequest{client: c}
}

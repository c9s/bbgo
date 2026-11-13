package backpackapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET

type StatusAndMessage struct {
	Status SystemStatus `json:"status"`

	// Message carries the maintenance notice when the status is not Ok.
	Message string `json:"message,omitempty"`
}

func (s StatusAndMessage) IsOk() bool {
	return s.Status == SystemStatusOk
}

//go:generate GetRequest -url "/api/v1/status" -type GetStatusRequest -responseType .StatusAndMessage
type GetStatusRequest struct {
	client requestgen.APIClient
}

func (c *RestClient) NewGetStatusRequest() *GetStatusRequest {
	return &GetStatusRequest{client: c}
}

package okexapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type AutoRepayResponse struct {
	AutoRepay bool `json:"autoRepay"`
}

//go:generate PostRequest -url "/api/v5/account/set-auto-repay" -type SetAutoRepayRequest -responseDataType []AutoRepayResponse
type SetAutoRepayRequest struct {
	client requestgen.AuthenticatedAPIClient

	autoRepay bool `param:"autoRepay"`
}

func (c *RestClient) NewSetAutoRepayRequest() *SetAutoRepayRequest {
	return &SetAutoRepayRequest{
		client: c,
	}
}

package okexapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type AccountLevelResponse struct {
	AcctLv string `json:"acctLv"`
}

//go:generate PostRequest -url "/api/v5/account/set-account-level" -type SetAccountLevelRequest -responseDataType []AccountLevelResponse
type SetAccountLevelRequest struct {
	client requestgen.AuthenticatedAPIClient

	acctLv string `param:"acctLv"`
}

func (c *RestClient) NewSetAccountLevelRequest() *SetAccountLevelRequest {
	return &SetAccountLevelRequest{
		client: c,
	}
}

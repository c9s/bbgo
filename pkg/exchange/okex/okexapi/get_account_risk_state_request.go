package okexapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type AccountRiskResponse struct {
	AtRisk    bool          `json:"atRisk"`
	AtRiskIdx []interface{} `json:"atRiskIdx"`
	AtRiskMgn []interface{} `json:"atRiskMgn"`
	Ts        string        `json:"ts"`
}

// GetAccountRiskStateRequest gets the account risk state
//
// Only applicable to Portfolio margin account
//
//go:generate GetRequest -url "/api/v5/account/risk-state" -type GetAccountRiskStateRequest -responseDataType []AccountRiskResponse
type GetAccountRiskStateRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountRiskStateRequest() *GetAccountRiskStateRequest {
	return &GetAccountRiskStateRequest{
		client: c,
	}
}

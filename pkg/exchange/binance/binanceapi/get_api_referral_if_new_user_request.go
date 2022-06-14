package binanceapi

import "github.com/c9s/requestgen"

type ApiReferralIfNewUserResponse struct {
	ApiAgentCode  string `json:"apiAgentCode"`
	RebateWorking bool   `json:"rebateWorking"`
	IfNewUser     bool   `json:"ifNewUser"`
	ReferrerId    int    `json:"referrerId"`
}

//go:generate requestgen -method GET -url "/sapi/v1/apiReferral/ifNewUser" -type GetApiReferralIfNewUserRequest -responseType .ApiReferralIfNewUserResponse
type GetApiReferralIfNewUserRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetApiReferralIfNewUserRequest() *GetApiReferralIfNewUserRequest {
	return &GetApiReferralIfNewUserRequest{client: c}
}

package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

// GetUserInfoRequest represents a Bitfinex user info request.
// API: https://docs.bitfinex.com/reference/rest-auth-info-user
//
//go:generate requestgen -type GetUserInfoRequest -method POST -url "/v2/auth/r/info/user" -responseType .UserInfoResponse
type GetUserInfoRequest struct {
	client requestgen.AuthenticatedAPIClient
}

// NewGetUserInfoRequest creates a new GetUserInfoRequest.
func (c *Client) NewGetUserInfoRequest() *GetUserInfoRequest {
	return &GetUserInfoRequest{client: c}
}

// UserInfoResponse represents the response from Bitfinex user info endpoint.
type UserInfoResponse struct {
	ID                               int64
	Email                            string
	Username                         string
	AccountCreatedAt                 types.MillisecondTimestamp
	Verified                         int
	VerificationLevel                int
	_                                any // placeholder
	Timezone                         string
	Locale                           string
	Company                          string
	EmailVerified                    int
	_                                any // placeholder
	SubaccountType                   *string
	_                                any // placeholder
	MasterAccountCreatedAt           types.MillisecondTimestamp
	GroupID                          int64
	MasterAccountID                  int64
	InheritMasterAccountVerification int
	IsGroupMaster                    int
	GroupWithdrawEnabled             int
	_4                               any // placeholder
	PPTEnabled                       *int
	MerchantEnabled                  int
	CompetitionEnabled               *int
	_5                               any // placeholder
	_6                               any // placeholder
	TwoFAModes                       []string
	_7                               any // placeholder
	IsSecuritiesMaster               int
	SecuritiesEnabled                *int
	IsSecuritiesInvestorAccredited   *int
	IsSecuritiesElSalvador           *int
	_8                               any // placeholder
	_9                               any // placeholder
	_10                              any // placeholder
	_11                              any // placeholder
	_12                              any // placeholder
	_13                              any // placeholder
	AllowDisableCtxSwitch            int
	CtxSwitchDisabled                int
	_14                              any // placeholder
	_15                              any // placeholder
	_16                              any // placeholder
	_17                              any // placeholder
	TimeLastLogin                    *string
	_18                              any // placeholder
	_19                              any // placeholder
	VerificationLevelSubmitted       *int
	_20                              any // placeholder
	CompCountries                    []string
	CompCountriesResid               []string
	ComplAccountType                 *string
	_21                              any // placeholder
	_22                              any // placeholder
	IsMerchantEnterprise             int
}

// UnmarshalJSON parses the Bitfinex user info response array.
func (r *UserInfoResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

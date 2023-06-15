package kucoinapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import "github.com/c9s/requestgen"

//go:generate GetRequest -url "/api/v1/accounts/:accountID" -type GetAccountRequest -responseDataType .Account
type GetAccountRequest struct {
	client    requestgen.AuthenticatedAPIClient
	accountID string `param:"accountID,slug"`
}

func (s *AccountService) NewGetAccountRequest(accountID string) *GetAccountRequest {
	return &GetAccountRequest{client: s.client, accountID: accountID}
}

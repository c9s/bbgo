package kucoinapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import "github.com/c9s/requestgen"

//go:generate GetRequest -url "/api/v1/accounts" -type ListAccountsRequest -responseDataType []Account
type ListAccountsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (s *AccountService) NewListAccountsRequest() *ListAccountsRequest {
	return &ListAccountsRequest{client: s.client}
}

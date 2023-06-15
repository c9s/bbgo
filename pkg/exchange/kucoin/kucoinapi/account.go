package kucoinapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type AccountService struct {
	client *RestClient
}

func (s *AccountService) NewListSubAccountsRequest() *ListSubAccountsRequest {
	return &ListSubAccountsRequest{client: s.client}
}

type SubAccount struct {
	UserID string `json:"userId"`
	Name   string `json:"subName"`
	Type   string `json:"type"`
	Remark string `json:"remarks"`
}

//go:generate GetRequest -url "/api/v1/sub/user" -type ListSubAccountsRequest -responseDataType []SubAccount
type ListSubAccountsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

type Account struct {
	ID        string           `json:"id"`
	Currency  string           `json:"currency"`
	Type      AccountType      `json:"type"`
	Balance   fixedpoint.Value `json:"balance"`
	Available fixedpoint.Value `json:"available"`
	Holds     fixedpoint.Value `json:"holds"`
}

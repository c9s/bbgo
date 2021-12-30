package kucoinapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method GET -responseType .APIResponse -responseDataField Data

import (
	"context"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type AccountService struct {
	client *RestClient
}

func (s *AccountService) ListSubAccounts(ctx context.Context) ([]SubAccount, error) {
	req := &NewListSubAccountsRequest{client: s.client}
	return req.Do(ctx)
}

func (s *AccountService) ListAccounts(ctx context.Context) ([]Account, error) {
	req := &NewListAccountsRequest{client: s.client}
	return req.Do(ctx)
}

func (s *AccountService) GetAccount(ctx context.Context, accountID string) (*Account, error) {
	req := &NewGetAccountRequest{client: s.client, accountID: accountID}
	return req.Do(ctx)
}

type SubAccount struct {
	UserID string `json:"userId"`
	Name   string `json:"subName"`
	Type   string `json:"type"`
	Remark string `json:"remarks"`
}

//go:generate GetRequest -url "/api/v1/sub/user" -type NewListSubAccountsRequest -responseDataType []SubAccount
type NewListSubAccountsRequest struct {
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

//go:generate GetRequest -url "/api/v1/accounts" -type NewListAccountsRequest -responseDataType []Account
type NewListAccountsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

//go:generate GetRequest -url "/api/v1/accounts/:accountID" -type NewGetAccountRequest -responseDataType .Account
type NewGetAccountRequest struct {
	client    requestgen.AuthenticatedAPIClient
	accountID string `param:"accountID,slug"`
}

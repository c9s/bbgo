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
	req := &ListSubAccountsRequest{client: s.client}
	return req.Do(ctx)
}

func (s *AccountService) ListAccounts(ctx context.Context) ([]Account, error) {
	req := &ListAccountsRequest{client: s.client}
	return req.Do(ctx)
}

func (s *AccountService) GetAccount(ctx context.Context, accountID string) (*Account, error) {
	req := &GetAccountRequest{client: s.client, accountID: accountID}
	return req.Do(ctx)
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

//go:generate GetRequest -url "/api/v1/accounts" -type ListAccountsRequest -responseDataType []Account
type ListAccountsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

//go:generate GetRequest -url "/api/v1/accounts/:accountID" -type GetAccountRequest -responseDataType .Account
type GetAccountRequest struct {
	client    requestgen.AuthenticatedAPIClient
	accountID string `param:"accountID,slug"`
}

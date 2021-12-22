package kucoinapi

import "github.com/c9s/bbgo/pkg/fixedpoint"

type AccountService struct {
	client *RestClient
}

type SubAccount struct {
	UserID string `json:"userId"`
	Name   string `json:"subName"`
	Type   string `json:"type"`
	Remark string `json:"remarks"`
}

func (s *AccountService) QuerySubAccounts() ([]SubAccount, error) {
	req, err := s.client.NewAuthenticatedRequest("GET", "/api/v1/sub/user", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string       `json:"code"`
		Message string       `json:"msg"`
		Data    []SubAccount `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}

type Account struct {
	ID        string           `json:"id"`
	Currency  string           `json:"currency"`
	Type      AccountType      `json:"type"`
	Balance   fixedpoint.Value `json:"balance"`
	Available fixedpoint.Value `json:"available"`
	Holds     fixedpoint.Value `json:"holds"`
}

func (s *AccountService) ListAccounts() ([]Account, error) {
	req, err := s.client.NewAuthenticatedRequest("GET", "/api/v1/accounts", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string    `json:"code"`
		Message string    `json:"msg"`
		Data    []Account `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}

func (s *AccountService) GetAccount(accountID string) (*Account, error) {
	req, err := s.client.NewAuthenticatedRequest("GET", "/api/v1/accounts/"+accountID, nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string   `json:"code"`
		Message string   `json:"msg"`
		Data    *Account `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}

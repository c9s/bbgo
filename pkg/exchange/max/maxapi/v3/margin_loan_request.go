package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import "github.com/c9s/requestgen"

func (s *Client) NewMarginLoanRequest() *MarginLoanRequest {
	return &MarginLoanRequest{client: s.Client}
}

//go:generate PostRequest -url "/api/v3/wallet/m/loan" -type MarginLoanRequest -responseType .LoanRecord
type MarginLoanRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string `param:"currency,required"`
	amount   string `param:"amount"`
}

package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import "github.com/c9s/requestgen"

//go:generate PostRequest -url "/api/v3/wallet/m/repayment" -type MarginRepayRequest -responseType .RepaymentRecord
type MarginRepayRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string `param:"currency,required"`
	amount   string `param:"amount"`
}

func (s *Client) NewMarginRepayRequest() *MarginRepayRequest {
	return &MarginRepayRequest{client: s.Client}
}

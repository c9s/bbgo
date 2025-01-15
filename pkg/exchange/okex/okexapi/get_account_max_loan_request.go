package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type MaxLoanResponse struct {
	InstId  string     `json:"instId"`
	MgnMode MarginMode `json:"mgnMode"`
	MgnCcy  string     `json:"mgnCcy"`

	MaxLoan fixedpoint.Value `json:"maxLoan"`
	Ccy     string           `json:"ccy"`
	Side    MarginSide       `json:"side"`
}

//go:generate GetRequest -url "/api/v5/account/max-loan" -type GetAccountMaxLoanRequest -responseDataType []MaxLoanResponse -rateLimiter 1+20/2s
type GetAccountMaxLoanRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentId   *string `param:"instId"`
	currency       *string `param:"ccy"`
	marginCurrency *string `param:"mgnCcy"`
}

func (c *RestClient) NewGetAccountMaxLoanRequest() *GetAccountMaxLoanRequest {
	return &GetAccountMaxLoanRequest{
		client: c,
	}
}

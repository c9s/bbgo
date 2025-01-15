package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type BorrowInterestLimit struct {
	Debt             fixedpoint.Value           `json:"debt"`
	Interest         fixedpoint.Value           `json:"interest"`
	LoanAlloc        fixedpoint.Value           `json:"loanAlloc"`
	NextDiscountTime types.MillisecondTimestamp `json:"nextDiscountTime"`
	NextInterestTime types.MillisecondTimestamp `json:"nextInterestTime"`
	Records          []struct {
		Currency  string           `json:"ccy"`
		AvailLoan fixedpoint.Value `json:"availLoan"`
		AvgRate   fixedpoint.Value `json:"avgRate"`
		Interest  fixedpoint.Value `json:"interest"`
		LoanQuota fixedpoint.Value `json:"loanQuota"`
		PosLoan   fixedpoint.Value `json:"posLoan"`
		Rate      fixedpoint.Value `json:"rate"`

		SurplusLimit        fixedpoint.Value `json:"surplusLmt"`
		SurplusLimitDetails struct {
		} `json:"surplusLmtDetails"`
		UsedLimit fixedpoint.Value `json:"usedLmt"`
		UsedLoan  fixedpoint.Value `json:"usedLoan"`
	} `json:"records"`
}

//go:generate GetRequest -url "/api/v5/account/interest-limits" -type GetAccountInterestLimitsRequest -responseDataType []BorrowInterestLimit -rateLimiter 1+20/2s
type GetAccountInterestLimitsRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency *string `param:"ccy"`
}

func (c *RestClient) NewGetAccountInterestLimitsRequest() *GetAccountInterestLimitsRequest {
	return &GetAccountInterestLimitsRequest{
		client: c,
	}
}

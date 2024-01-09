package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type BalanceDetail struct {
	Currency                string                     `json:"ccy"`
	Available               fixedpoint.Value           `json:"availEq"`
	CashBalance             fixedpoint.Value           `json:"cashBal"`
	OrderFrozen             fixedpoint.Value           `json:"ordFrozen"`
	Frozen                  fixedpoint.Value           `json:"frozenBal"`
	Equity                  fixedpoint.Value           `json:"eq"`
	EquityInUSD             fixedpoint.Value           `json:"eqUsd"`
	UpdateTime              types.MillisecondTimestamp `json:"uTime"`
	UnrealizedProfitAndLoss fixedpoint.Value           `json:"upl"`
}

type Account struct {
	TotalEquityInUSD fixedpoint.Value           `json:"totalEq"`
	UpdateTime       types.MillisecondTimestamp `json:"uTime"`
	Details          []BalanceDetail            `json:"details"`
}

//go:generate GetRequest -url "/api/v5/account/balance" -type GetAccountInfoRequest -responseDataType []Account
type GetAccountInfoRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountInfoRequest() *GetAccountInfoRequest {
	return &GetAccountInfoRequest{
		client: c,
	}
}

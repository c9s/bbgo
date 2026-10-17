package okexapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type OneClickRepayResponse struct {
	DebtCcy     string `json:"debtCcy"`
	RepayCcy    string `json:"repayCcy"`
	RepayCcyAmt string `json:"repayCcyAmt"`
	RepayAmt    string `json:"repayAmt"`
	Ts          string `json:"ts,omitempty"`
}

// One-click repay to repay cross debts on a multi-currency margin account.
// No amount parameter: repays the full debt up to the repay currency's
// available balance.
//go:generate PostRequest -url "/api/v5/trade/one-click-repay" -type OneClickRepayRequest -responseDataType []OneClickRepayResponse -rateLimiter 1+10/2s
type OneClickRepayRequest struct {
	client requestgen.AuthenticatedAPIClient

	// debtCcy: debt currency (max 5, comma separated)
	debtCurrency string `param:"debtCcy"`

	// repayCcy: currency used to repay the debt
	repayCurrency string `param:"repayCcy"`
}

func (c *RestClient) NewOneClickRepayRequest() *OneClickRepayRequest {
	return &OneClickRepayRequest{
		client: c,
	}
}

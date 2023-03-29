package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type FuturesBalance struct {
	AccountAlias       string                     `json:"accountAlias"`
	Asset              string                     `json:"asset"`
	Balance            fixedpoint.Value           `json:"balance"`
	CrossWalletBalance fixedpoint.Value           `json:"crossWalletBalance"`
	CrossUnPnl         fixedpoint.Value           `json:"crossUnPnl"`
	AvailableBalance   fixedpoint.Value           `json:"availableBalance"`
	MaxWithdrawAmount  fixedpoint.Value           `json:"maxWithdrawAmount"`
	MarginAvailable    bool                       `json:"marginAvailable"`
	UpdateTime         types.MillisecondTimestamp `json:"updateTime"`
}

//go:generate requestgen -method GET -url "/fapi/v2/balance" -type FuturesGetAccountBalanceRequest -responseType []FuturesBalance
type FuturesGetAccountBalanceRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *FuturesRestClient) NewFuturesGetAccountBalanceRequest() *FuturesGetAccountBalanceRequest {
	return &FuturesGetAccountBalanceRequest{client: c}
}

package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type FuturesBalance struct {
	AccountAlias string `json:"accountAlias"`
	Asset        string `json:"asset"`

	// Balance - wallet balance
	Balance fixedpoint.Value `json:"balance"`

	CrossWalletBalance fixedpoint.Value `json:"crossWalletBalance"`

	// CrossUnPnL unrealized profit of crossed positions
	CrossUnPnl fixedpoint.Value `json:"crossUnPnl"`

	AvailableBalance fixedpoint.Value `json:"availableBalance"`

	// MaxWithdrawAmount - maximum amount for transfer out
	MaxWithdrawAmount fixedpoint.Value `json:"maxWithdrawAmount"`

	// MarginAvailable - whether the asset can be used as margin in Multi-Assets mode
	MarginAvailable bool                       `json:"marginAvailable"`
	UpdateTime      types.MillisecondTimestamp `json:"updateTime"`
}

//go:generate requestgen -method GET -url "/fapi/v2/balance" -type FuturesGetAccountBalanceRequest -responseType []FuturesBalance
type FuturesGetAccountBalanceRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *FuturesRestClient) NewFuturesGetAccountBalanceRequest() *FuturesGetAccountBalanceRequest {
	return &FuturesGetAccountBalanceRequest{client: c}
}

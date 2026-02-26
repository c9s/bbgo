package hyperapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Balance struct {
	Coin     string           `json:"coin"`
	Token    int64            `json:"token"`
	Hold     fixedpoint.Value `json:"hold"`
	Total    fixedpoint.Value `json:"total"`
	EntryNtl fixedpoint.Value `json:"entryNtl"`
}

type Account struct {
	Balances []Balance `json:"balances"`
}

//go:generate requestgen -method POST -url "/info" -type GetAccountBalanceRequest -responseType Account
type GetAccountBalanceRequest struct {
	client requestgen.APIClient

	user     string      `param:"user,required"`
	metaType ReqTypeInfo `param:"type" default:"spotClearinghouseState" validValues:"spotClearinghouseState"`
}

func (c *Client) NewGetAccountBalanceRequest() *GetAccountBalanceRequest {
	return &GetAccountBalanceRequest{
		client:   c,
		metaType: ReqSpotClearinghouseState,
	}
}

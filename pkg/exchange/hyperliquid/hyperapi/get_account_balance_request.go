package hyperapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Response.Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Response.Data

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

//go:generate GetRequest -url "/info" -type GetAccountBalanceRequest -responseDataType Account
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

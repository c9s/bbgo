package coinbase

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/requestgen"
)

type Balance struct {
	ID             string           `json:"id"`
	Currency       string           `json:"currency"`
	Balance        fixedpoint.Value `json:"balance"`
	Hold           fixedpoint.Value `json:"hold"`
	Available      fixedpoint.Value `json:"available"`
	ProfileID      string           `json:"profile_id"`
	TradingEnabled bool             `json:"trading_enabled"`
}

type BalanceSnapshot []Balance

//go:generate requestgen -method GET -url /accounts -type GetBalancesRequest -responseType .BalanceSnapshot
type GetBalancesRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestAPIClient) NewGetBalancesRequest() *GetBalancesRequest {
	return &GetBalancesRequest{client: c}
}

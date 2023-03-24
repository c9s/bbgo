package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type FuturesPositionRisk struct {
	EntryPrice       string                     `json:"entryPrice"`
	MarginType       string                     `json:"marginType"`
	IsAutoAddMargin  string                     `json:"isAutoAddMargin"`
	IsolatedMargin   string                     `json:"isolatedMargin"`
	Leverage         fixedpoint.Value           `json:"leverage"`
	LiquidationPrice fixedpoint.Value           `json:"liquidationPrice"`
	MarkPrice        fixedpoint.Value           `json:"markPrice"`
	MaxNotionalValue fixedpoint.Value           `json:"maxNotionalValue"`
	PositionAmount   fixedpoint.Value           `json:"positionAmt"`
	Notional         fixedpoint.Value           `json:"notional"`
	IsolatedWallet   string                     `json:"isolatedWallet"`
	Symbol           string                     `json:"symbol"`
	UnRealizedProfit fixedpoint.Value           `json:"unRealizedProfit"`
	PositionSide     string                     `json:"positionSide"`
	UpdateTime       types.MillisecondTimestamp `json:"updateTime"`
}

//go:generate requestgen -method GET -url "/fapi/v2/positionRisk" -type FuturesGetPositionRisksRequest -responseType []FuturesPositionRisk
type FuturesGetPositionRisksRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol"`
}

func (c *FuturesRestClient) NewGetPositionRisksRequest() *FuturesGetPositionRisksRequest {
	return &FuturesGetPositionRisksRequest{client: c}
}

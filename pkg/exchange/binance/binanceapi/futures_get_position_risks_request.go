package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type FuturesPositionRisk struct {
	EntryPrice       fixedpoint.Value           `json:"entryPrice"`
	IsolatedMargin   string                     `json:"isolatedMargin"`
	LiquidationPrice fixedpoint.Value           `json:"liquidationPrice"`
	MarkPrice        fixedpoint.Value           `json:"markPrice"`
	PositionAmount   fixedpoint.Value           `json:"positionAmt"`
	Notional         fixedpoint.Value           `json:"notional"`
	IsolatedWallet   string                     `json:"isolatedWallet"`
	Symbol           string                     `json:"symbol"`
	UnRealizedProfit fixedpoint.Value           `json:"unRealizedProfit"`
	PositionSide     string                     `json:"positionSide"`
	UpdateTime       types.MillisecondTimestamp `json:"updateTime"`

	BreakEvenPrice         fixedpoint.Value `json:"breakEvenPrice"`
	MarginAsset            fixedpoint.Value `json:"marginAsset"`
	InitialMargin          fixedpoint.Value `json:"initialMargin"`
	MaintMargin            fixedpoint.Value `json:"maintMargin"`
	PositionInitialMargin  fixedpoint.Value `json:"positionInitialMargin"`
	OpenOrderInitialMargin fixedpoint.Value `json:"openOrderInitialMargin"`
	Adl                    fixedpoint.Value `json:"adl"`
	BidNotional            fixedpoint.Value `json:"bidNotional"`
	AskNotional            fixedpoint.Value `json:"askNotional"`
}

//go:generate requestgen -method GET -url "/fapi/v3/positionRisk" -type FuturesGetPositionRisksRequest -responseType []FuturesPositionRisk
type FuturesGetPositionRisksRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol"`
}

func (c *FuturesRestClient) NewFuturesGetPositionRisksRequest() *FuturesGetPositionRisksRequest {
	return &FuturesGetPositionRisksRequest{client: c}
}

package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type FuturesPositionRisk struct {
	Symbol                 string           `json:"symbol"`
	PositionSide           string           `json:"positionSide"`
	PositionAmount         fixedpoint.Value `json:"positionAmt"`
	EntryPrice             fixedpoint.Value `json:"entryPrice"`
	BreakEvenPrice         fixedpoint.Value `json:"breakEvenPrice"`
	MarkPrice              fixedpoint.Value `json:"markPrice"`
	UnRealizedProfit       fixedpoint.Value `json:"unRealizedProfit"`
	LiquidationPrice       fixedpoint.Value `json:"liquidationPrice"`
	IsolatedMargin         string           `json:"isolatedMargin"`
	Notional               fixedpoint.Value `json:"notional"`
	MarginAsset            fixedpoint.Value `json:"marginAsset"`
	IsolatedWallet         string           `json:"isolatedWallet"`
	InitialMargin          fixedpoint.Value `json:"initialMargin"`
	MaintMargin            fixedpoint.Value `json:"maintMargin"`
	PositionInitialMargin  fixedpoint.Value `json:"positionInitialMargin"`
	OpenOrderInitialMargin fixedpoint.Value `json:"openOrderInitialMargin"`
	Adl                    fixedpoint.Value `json:"adl"`
	BidNotional            fixedpoint.Value `json:"bidNotional"`
	AskNotional            fixedpoint.Value `json:"askNotional"`

	UpdateTime types.MillisecondTimestamp `json:"updateTime"`
}

//go:generate requestgen -method GET -url "/fapi/v3/positionRisk" -type FuturesGetPositionRisksRequest -responseType []FuturesPositionRisk
type FuturesGetPositionRisksRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol"`
}

func (c *FuturesRestClient) NewFuturesGetPositionRisksRequest() *FuturesGetPositionRisksRequest {
	return &FuturesGetPositionRisksRequest{client: c}
}

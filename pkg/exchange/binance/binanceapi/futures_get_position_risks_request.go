package binanceapi

import "github.com/c9s/requestgen"

type FuturesPositionRisk struct {
	EntryPrice       string `json:"entryPrice"`
	MarginType       string `json:"marginType"`
	IsAutoAddMargin  string `json:"isAutoAddMargin"`
	IsolatedMargin   string `json:"isolatedMargin"`
	Leverage         string `json:"leverage"`
	LiquidationPrice string `json:"liquidationPrice"`
	MarkPrice        string `json:"markPrice"`
	MaxNotionalValue string `json:"maxNotionalValue"`
	PositionAmt      string `json:"positionAmt"`
	Notional         string `json:"notional"`
	IsolatedWallet   string `json:"isolatedWallet"`
	Symbol           string `json:"symbol"`
	UnRealizedProfit string `json:"unRealizedProfit"`
	PositionSide     string `json:"positionSide"`
	UpdateTime       int    `json:"updateTime"`
}

//go:generate requestgen -method GET -url "/fapi/v2/positionRisk" -type FuturesGetPositionRisksRequest -responseType []FuturesPositionRisk
type FuturesGetPositionRisksRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol"`
}

func (c *FuturesRestClient) NewGetPositionRisksRequest() *FuturesGetPositionRisksRequest {
	return &FuturesGetPositionRisksRequest{client: c}
}

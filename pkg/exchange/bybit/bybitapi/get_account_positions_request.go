package bybitapi

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type Position struct {
	// Position idx, used to identify positions in different position modes
	PositionIdx int `json:"positionIdx"`

	// Risk tier ID
	RiskId int `json:"riskId"`

	// Risk limit value
	RiskLimitValue string `json:"riskLimitValue"`

	// Symbol name
	Symbol string `json:"symbol"`

	// Position side. Buy: long, Sell: short
	Side string `json:"side"`

	// Position size, always positive (string in API)
	Size string `json:"size"`

	// Average entry price
	AvgPrice string `json:"avgPrice"`

	// Position value
	PositionValue string `json:"positionValue"`

	// Trade mode (0: cross, 1: isolated)
	TradeMode int `json:"tradeMode"`

	// Position status. Normal, Liq, Adl
	PositionStatus string `json:"positionStatus"`

	// Whether to add margin automatically when isolated
	AutoAddMargin int `json:"autoAddMargin"`

	// Auto-deleverage rank indicator
	AdlRankIndicator int `json:"adlRankIndicator"`

	// Position leverage (string in API)
	Leverage string `json:"leverage"`

	// Position margin (initial margin plus closing fee for some accounts)
	PositionBalance string `json:"positionBalance"`

	// Mark price
	MarkPrice string `json:"markPrice"`

	// Liquidation price
	LiqPrice string `json:"liqPrice"`

	// Bankruptcy price (classic accounts)
	BustPrice string `json:"bustPrice"`

	// Maintenance margin
	PositionMM string `json:"positionMM"`

	// Maintenance margin calculated by mark price
	PositionMMByMp string `json:"positionMMByMp"`

	// Initial margin
	PositionIM string `json:"positionIM"`

	// Initial margin calculated by mark price
	PositionIMByMp string `json:"positionIMByMp"`

	// Take profit price
	TakeProfit string `json:"takeProfit"`

	// Stop loss price
	StopLoss string `json:"stopLoss"`

	// Trailing stop (distance from market price)
	TrailingStop string `json:"trailingStop"`

	// Greeks (option)
	Delta string `json:"delta,omitempty"`
	Gamma string `json:"gamma,omitempty"`
	Vega  string `json:"vega,omitempty"`
	Theta string `json:"theta,omitempty"`

	// Unrealised PnL
	UnrealisedPnl string `json:"unrealisedPnl"`

	// Realised PnL for the current holding position
	CurRealisedPnl string `json:"curRealisedPnl"`

	// Cumulative realised PnL
	CumRealisedPnl string `json:"cumRealisedPnl"`

	// Cross sequence
	Seq int64 `json:"seq"`

	// Reduce-only flag
	IsReduceOnly bool `json:"isReduceOnly"`

	// System MMR update timestamp (field name per API example)
	MmrSysUpdateTime string `json:"mmrSysUpdateTime,omitempty"`

	// Leverage system updated time
	LeverageSysUpdatedTime string `json:"leverageSysUpdatedTime"`

	// USDC contract session avg price
	SessionAvgPrice string `json:"sessionAvgPrice"`

	// First time a position was created on this symbol (ms)
	CreatedTime string `json:"createdTime"`

	// Position updated timestamp (ms)
	UpdatedTime string `json:"updatedTime"`

	// Deprecated, always "Full"
	TpslMode string `json:"tpslMode"`
}

type PositionsResponse struct {
	List           []Position `json:"list"`
	NextPageCursor string     `json:"nextPageCursor"`
	Category       string     `json:"category"`
}

type GetAccountPositionsRequest struct {
	client requestgen.AuthenticatedAPIClient

	category Category `param:"category,query" validValues:"inverse,linear"`
	symbol   *string  `param:"symbol,query"`

	baseCoin   *string `param:"baseCoin,query"`
	settleCoin *string `param:"settleCoin,query" default:"USDT"`

	// limit for data size per page. [1, 200]. Default: 20
	limit *uint64 `param:"limit,query" default:"200"`

	// cursor uses the nextPageCursor token from the response to retrieve the next page of the result set
	cursor *string `param:"cursor,query"`
}

//go:generate GetRequest -url "/v5/position/list" -type GetAccountPositionsRequest -responseDataType .PositionsResponse
func (c *RestClient) NewGetAccountPositionsRequest() *GetAccountPositionsRequest {
	return &GetAccountPositionsRequest{
		client:   c,
		category: CategoryLinear,
	}
}

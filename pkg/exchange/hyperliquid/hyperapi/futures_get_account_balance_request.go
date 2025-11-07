package hyperapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/requestgen"
)

type FuturesAccount struct {
	AssetPositions             []FuturesAssetPosition `json:"assetPositions"`
	CrossMaintenanceMarginUsed fixedpoint.Value       `json:"crossMaintenanceMarginUsed"`
	CrossMarginSummary         FuturesMarginSummary   `json:"crossMarginSummary"`
	MarginSummary              FuturesMarginSummary   `json:"marginSummary"`
	Time                       int64                  `json:"time"`
	Withdrawable               fixedpoint.Value       `json:"withdrawable"`
}

type FuturesAssetPosition struct {
	Position FuturesPosition `json:"position"`
	Type     string          `json:"type"`
}

type FuturesPosition struct {
	Coin           string            `json:"coin"`
	CumFunding     FuturesCumFunding `json:"cumFunding"`
	EntryPx        fixedpoint.Value  `json:"entryPx"`
	Leverage       FuturesLeverage   `json:"leverage"`
	LiquidationPx  fixedpoint.Value  `json:"liquidationPx"`
	MarginUsed     fixedpoint.Value  `json:"marginUsed"`
	MaxLeverage    int               `json:"maxLeverage"`
	PositionValue  fixedpoint.Value  `json:"positionValue"`
	ReturnOnEquity fixedpoint.Value  `json:"returnOnEquity"`
	Szi            fixedpoint.Value  `json:"szi"`
	UnrealizedPnl  fixedpoint.Value  `json:"unrealizedPnl"`
}

type FuturesCumFunding struct {
	AllTime     fixedpoint.Value `json:"allTime"`
	SinceChange fixedpoint.Value `json:"sinceChange"`
	SinceOpen   fixedpoint.Value `json:"sinceOpen"`
}

type FuturesLeverage struct {
	RawUsd fixedpoint.Value `json:"rawUsd"`
	Type   string           `json:"type"`
	Value  fixedpoint.Value `json:"value"`
}

type FuturesMarginSummary struct {
	AccountValue    fixedpoint.Value `json:"accountValue"`
	TotalMarginUsed fixedpoint.Value `json:"totalMarginUsed"`
	TotalNtlPos     fixedpoint.Value `json:"totalNtlPos"`
	TotalRawUsd     fixedpoint.Value `json:"totalRawUsd"`
}

//go:generate requestgen -method POST -url "/info" -type FuturesGetAccountBalanceRequest -responseType FuturesAccount
type FuturesGetAccountBalanceRequest struct {
	client requestgen.APIClient

	user     string      `param:"user,required"`
	dex      *string     `param:"dex"`
	metaType ReqTypeInfo `param:"type" default:"clearinghouseState" validValues:"clearinghouseState"`
}

func (c *Client) NewFuturesGetAccountBalanceRequest() *FuturesGetAccountBalanceRequest {
	return &FuturesGetAccountBalanceRequest{
		client:   c,
		metaType: ReqFuturesClearinghouseState,
	}
}

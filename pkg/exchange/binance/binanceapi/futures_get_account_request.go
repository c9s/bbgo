package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// FuturesAccountAsset define account asset
type FuturesAccountAsset struct {
	Asset string `json:"asset"`

	InitialMargin          fixedpoint.Value `json:"initialMargin"`
	MaintMargin            fixedpoint.Value `json:"maintMargin"`
	MarginBalance          fixedpoint.Value `json:"marginBalance"`
	MaxWithdrawAmount      fixedpoint.Value `json:"maxWithdrawAmount"`
	OpenOrderInitialMargin fixedpoint.Value `json:"openOrderInitialMargin"`
	PositionInitialMargin  fixedpoint.Value `json:"positionInitialMargin"`
	UnrealizedProfit       fixedpoint.Value `json:"unrealizedProfit"`
	WalletBalance          fixedpoint.Value `json:"walletBalance"`
}

// FuturesAccountPosition define account position
type FuturesAccountPosition struct {
	Isolated               bool             `json:"isolated"`
	Leverage               fixedpoint.Value `json:"leverage"`
	InitialMargin          fixedpoint.Value `json:"initialMargin"`
	MaintMargin            fixedpoint.Value `json:"maintMargin"`
	OpenOrderInitialMargin fixedpoint.Value `json:"openOrderInitialMargin"`
	PositionInitialMargin  fixedpoint.Value `json:"positionInitialMargin"`
	Symbol                 string           `json:"symbol"`
	UnrealizedProfit       fixedpoint.Value `json:"unrealizedProfit"`
	EntryPrice             fixedpoint.Value `json:"entryPrice"`
	BreakEvenPrice         fixedpoint.Value `json:"breakEvenPrice"`
	MaxNotional            fixedpoint.Value `json:"maxNotional"`
	PositionSide           string           `json:"positionSide"`
	PositionAmt            fixedpoint.Value `json:"positionAmt"`
	Notional               fixedpoint.Value `json:"notional"`
	IsolatedWallet         string           `json:"isolatedWallet"`
	UpdateTime             int64            `json:"updateTime"`
}

type FuturesAccount struct {
	FeeTier     int  `json:"feeTier"`
	FeeBurn     bool `json:"feeBurn"`
	CanTrade    bool `json:"canTrade"`
	CanDeposit  bool `json:"canDeposit"`
	CanWithdraw bool `json:"canWithdraw"`

	// reserved property, please ignore
	UpdateTime int64 `json:"updateTime"`

	MultiAssetsMargin           bool                     `json:"multiAssetsMargin"`
	TradeGroupId                int                      `json:"tradeGroupId"`
	TotalInitialMargin          fixedpoint.Value         `json:"totalInitialMargin"`
	TotalMaintMargin            fixedpoint.Value         `json:"totalMaintMargin"`
	TotalWalletBalance          fixedpoint.Value         `json:"totalWalletBalance"`
	TotalUnrealizedProfit       fixedpoint.Value         `json:"totalUnrealizedProfit"`
	TotalMarginBalance          fixedpoint.Value         `json:"totalMarginBalance"`
	TotalPositionInitialMargin  fixedpoint.Value         `json:"totalPositionInitialMargin"`
	TotalOpenOrderInitialMargin fixedpoint.Value         `json:"totalOpenOrderInitialMargin"`
	TotalCrossWalletBalance     fixedpoint.Value         `json:"totalCrossWalletBalance"`
	TotalCrossUnPnl             fixedpoint.Value         `json:"totalCrossUnPnl"`
	AvailableBalance            fixedpoint.Value         `json:"availableBalance"`
	MaxWithdrawAmount           fixedpoint.Value         `json:"maxWithdrawAmount"`
	Assets                      []FuturesAccountAsset    `json:"assets"`
	Positions                   []FuturesAccountPosition `json:"positions"`
}

//go:generate requestgen -method GET -url "/fapi/v2/account" -type FuturesGetAccountRequest -responseType FuturesAccount
type FuturesGetAccountRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *FuturesRestClient) NewFuturesGetAccountRequest() *FuturesGetAccountRequest {
	return &FuturesGetAccountRequest{client: c}
}

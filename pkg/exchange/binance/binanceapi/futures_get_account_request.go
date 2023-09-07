package binanceapi

import (
	"github.com/c9s/requestgen"
)

// FuturesAccountAsset define account asset
type FuturesAccountAsset struct {
	Asset                  string `json:"asset"`
	InitialMargin          string `json:"initialMargin"`
	MaintMargin            string `json:"maintMargin"`
	MarginBalance          string `json:"marginBalance"`
	MaxWithdrawAmount      string `json:"maxWithdrawAmount"`
	OpenOrderInitialMargin string `json:"openOrderInitialMargin"`
	PositionInitialMargin  string `json:"positionInitialMargin"`
	UnrealizedProfit       string `json:"unrealizedProfit"`
	WalletBalance          string `json:"walletBalance"`
}

// FuturesAccountPosition define account position
type FuturesAccountPosition struct {
	Isolated               bool   `json:"isolated"`
	Leverage               string `json:"leverage"`
	InitialMargin          string `json:"initialMargin"`
	MaintMargin            string `json:"maintMargin"`
	OpenOrderInitialMargin string `json:"openOrderInitialMargin"`
	PositionInitialMargin  string `json:"positionInitialMargin"`
	Symbol                 string `json:"symbol"`
	UnrealizedProfit       string `json:"unrealizedProfit"`
	EntryPrice             string `json:"entryPrice"`
	MaxNotional            string `json:"maxNotional"`
	PositionSide           string `json:"positionSide"`
	PositionAmt            string `json:"positionAmt"`
	Notional               string `json:"notional"`
	IsolatedWallet         string `json:"isolatedWallet"`
	UpdateTime             int64  `json:"updateTime"`
}

type FuturesAccount struct {
	Assets                      []*FuturesAccountAsset    `json:"assets"`
	FeeTier                     int                       `json:"feeTier"`
	CanTrade                    bool                      `json:"canTrade"`
	CanDeposit                  bool                      `json:"canDeposit"`
	CanWithdraw                 bool                      `json:"canWithdraw"`
	UpdateTime                  int64                     `json:"updateTime"`
	TotalInitialMargin          string                    `json:"totalInitialMargin"`
	TotalMaintMargin            string                    `json:"totalMaintMargin"`
	TotalWalletBalance          string                    `json:"totalWalletBalance"`
	TotalUnrealizedProfit       string                    `json:"totalUnrealizedProfit"`
	TotalMarginBalance          string                    `json:"totalMarginBalance"`
	TotalPositionInitialMargin  string                    `json:"totalPositionInitialMargin"`
	TotalOpenOrderInitialMargin string                    `json:"totalOpenOrderInitialMargin"`
	TotalCrossWalletBalance     string                    `json:"totalCrossWalletBalance"`
	TotalCrossUnPnl             string                    `json:"totalCrossUnPnl"`
	AvailableBalance            string                    `json:"availableBalance"`
	MaxWithdrawAmount           string                    `json:"maxWithdrawAmount"`
	Positions                   []*FuturesAccountPosition `json:"positions"`
}

//go:generate requestgen -method GET -url "/fapi/v2/account" -type FuturesGetAccountRequest -responseType FuturesAccount
type FuturesGetAccountRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *FuturesRestClient) NewFuturesGetAccountRequest() *FuturesGetAccountRequest {
	return &FuturesGetAccountRequest{client: c}
}

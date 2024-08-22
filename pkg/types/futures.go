package types

import "github.com/c9s/bbgo/pkg/fixedpoint"

type FuturesExchange interface {
	UseFutures()
	UseIsolatedFutures(symbol string)
	GetFuturesSettings() FuturesSettings
}

type FuturesSettings struct {
	IsFutures             bool
	IsIsolatedFutures     bool
	IsolatedFuturesSymbol string
}

func (s FuturesSettings) GetFuturesSettings() FuturesSettings {
	return s
}

func (s *FuturesSettings) UseFutures() {
	s.IsFutures = true
}

func (s *FuturesSettings) UseIsolatedFutures(symbol string) {
	s.IsFutures = true
	s.IsIsolatedFutures = true
	s.IsolatedFuturesSymbol = symbol
}

// FuturesUserAsset define cross/isolated futures account asset
type FuturesUserAsset struct {
	Asset                  string           `json:"asset"`
	InitialMargin          fixedpoint.Value `json:"initialMargin"`
	MaintMargin            fixedpoint.Value `json:"maintMargin"`
	MarginBalance          fixedpoint.Value `json:"marginBalance"`
	MaxWithdrawAmount      fixedpoint.Value `json:"maxWithdrawAmount"`
	OpenOrderInitialMargin fixedpoint.Value `json:"openOrderInitialMargin"`
	PositionInitialMargin  fixedpoint.Value `json:"positionInitialMargin"`
	UnrealizedProfit       fixedpoint.Value `json:"unrealizedProfit"`
	WalletBalance          fixedpoint.Value `json:"walletBalance"`
}

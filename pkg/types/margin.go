package types

import "github.com/c9s/bbgo/pkg/fixedpoint"

type FuturesExchange interface {
	UseFutures()
	GetFuturesSettings() FuturesSettings
}

type FuturesSettings struct {
	IsFutures bool
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

// FuturesAccount is for the cross futures account
// type FuturesAccount struct {
// 	Assets                      []FuturesUserAsset    `json:"assets"`
// 	CanDeposit                  bool               `json:"canDeposit"`
// 	CanTrade                    bool               `json:"canTrade"`
// 	CanWithdraw                 bool               `json:"canWithdraw"`
// 	FeeTier                     int                `json:"feeTier"`
// 	MaxWithdrawAmount           fixedpoint.Value             `json:"maxWithdrawAmount"`
// 	Positions                   []FuturesUserPosition `json:"positions"`
// 	TotalInitialMargin          fixedpoint.Value             `json:"totalInitialMargin"`
// 	TotalMaintMargin            fixedpoint.Value             `json:"totalMaintMargin"`
// 	TotalMarginBalance          fixedpoint.Value             `json:"totalMarginBalance"`
// 	TotalOpenOrderInitialMargin fixedpoint.Value             `json:"totalOpenOrderInitialMargin"`
// 	TotalPositionInitialMargin  fixedpoint.Value             `json:"totalPositionInitialMargin"`
// 	TotalUnrealizedProfit       fixedpoint.Value             `json:"totalUnrealizedProfit"`
// 	TotalWalletBalance          fixedpoint.Value             `json:"totalWalletBalance"`
// 	UpdateTime                  int64              `json:"updateTime"`
// }

// FuturesUserAsset define cross futures account asset
type FuturesUserAsset struct {
	Asset                  string `json:"asset"`
	InitialMargin          fixedpoint.Value `json:"initialMargin"`
	MaintMargin            fixedpoint.Value `json:"maintMargin"`
	MarginBalance          fixedpoint.Value `json:"marginBalance"`
	MaxWithdrawAmount      fixedpoint.Value `json:"maxWithdrawAmount"`
	OpenOrderInitialMargin fixedpoint.Value `json:"openOrderInitialMargin"`
	PositionInitialMargin  fixedpoint.Value `json:"positionInitialMargin"`
	UnrealizedProfit       fixedpoint.Value `json:"unrealizedProfit"`
	WalletBalance          fixedpoint.Value `json:"walletBalance"`
}

// FuturesUserPosition define cross futures account position
// type FuturesUserPosition struct {
// 	Isolated               bool             `json:"isolated"`
// 	Leverage               fixedpoint.Value           `json:"leverage"`
// 	InitialMargin          fixedpoint.Value           `json:"initialMargin"`
// 	MaintMargin            fixedpoint.Value           `json:"maintMargin"`
// 	OpenOrderInitialMargin fixedpoint.Value           `json:"openOrderInitialMargin"`
// 	PositionInitialMargin  fixedpoint.Value           `json:"positionInitialMargin"`
// 	Symbol                 string           `json:"symbol"`
// 	UnrealizedProfit       fixedpoint.Value           `json:"unrealizedProfit"`
// 	EntryPrice             fixedpoint.Value           `json:"entryPrice"`
// 	MaxNotional            fixedpoint.Value           `json:"maxNotional"`
// 	PositionSide           string `json:"positionSide"`
// 	PositionAmt            fixedpoint.Value           `json:"positionAmt"`
// 	Notional               fixedpoint.Value           `json:"notional"`
// 	IsolatedWallet         fixedpoint.Value           `json:"isolatedWallet"`
// 	UpdateTime             int64            `json:"updateTime"`
// }

// IsolatedFuturesAccount is for the isolated futures account
// type IsolatedFuturesAccount struct {
// 	Assets                      []IsolatedFuturesUserAsset    `json:"assets"`
// 	CanDeposit                  bool               `json:"canDeposit"`
// 	CanTrade                    bool               `json:"canTrade"`
// 	CanWithdraw                 bool               `json:"canWithdraw"`
// 	FeeTier                     int                `json:"feeTier"`
// 	MaxWithdrawAmount           fixedpoint.Value             `json:"maxWithdrawAmount"`
// 	Positions                   []IsolatedFuturesUserPosition `json:"positions"`
// 	TotalInitialMargin          fixedpoint.Value             `json:"totalInitialMargin"`
// 	TotalMaintMargin            fixedpoint.Value             `json:"totalMaintMargin"`
// 	TotalMarginBalance          fixedpoint.Value             `json:"totalMarginBalance"`
// 	TotalOpenOrderInitialMargin fixedpoint.Value             `json:"totalOpenOrderInitialMargin"`
// 	TotalPositionInitialMargin  fixedpoint.Value             `json:"totalPositionInitialMargin"`
// 	TotalUnrealizedProfit       fixedpoint.Value             `json:"totalUnrealizedProfit"`
// 	TotalWalletBalance          fixedpoint.Value             `json:"totalWalletBalance"`
// 	UpdateTime                  int64              `json:"updateTime"`
// }

// IsolatedFuturesUserAsset define isolated futures account asset
type IsolatedFuturesUserAsset struct {
	Asset                  string `json:"asset"`
	InitialMargin          fixedpoint.Value `json:"initialMargin"`
	MaintMargin            fixedpoint.Value `json:"maintMargin"`
	MarginBalance          fixedpoint.Value `json:"marginBalance"`
	MaxWithdrawAmount      fixedpoint.Value `json:"maxWithdrawAmount"`
	OpenOrderInitialMargin fixedpoint.Value `json:"openOrderInitialMargin"`
	PositionInitialMargin  fixedpoint.Value `json:"positionInitialMargin"`
	UnrealizedProfit       fixedpoint.Value `json:"unrealizedProfit"`
	WalletBalance          fixedpoint.Value `json:"walletBalance"`
}

// IsolatedFuturesUserPosition define isolated futures account position
// type IsolatedFuturesUserPosition struct {
// 	Isolated               bool             `json:"isolated"`
// 	Leverage               fixedpoint.Value           `json:"leverage"`
// 	InitialMargin          fixedpoint.Value           `json:"initialMargin"`
// 	MaintMargin            fixedpoint.Value           `json:"maintMargin"`
// 	OpenOrderInitialMargin fixedpoint.Value           `json:"openOrderInitialMargin"`
// 	PositionInitialMargin  fixedpoint.Value           `json:"positionInitialMargin"`
// 	Symbol                 string           `json:"symbol"`
// 	UnrealizedProfit       fixedpoint.Value           `json:"unrealizedProfit"`
// 	EntryPrice             fixedpoint.Value           `json:"entryPrice"`
// 	MaxNotional            fixedpoint.Value           `json:"maxNotional"`
// 	PositionSide           string `json:"positionSide"`
// 	PositionAmt            fixedpoint.Value           `json:"positionAmt"`
// 	Notional               fixedpoint.Value           `json:"notional"`
// 	IsolatedWallet         fixedpoint.Value           `json:"isolatedWallet"`
// 	UpdateTime             int64            `json:"updateTime"`
// }

type MarginExchange interface {
	UseMargin()
	UseIsolatedMargin(symbol string)
	GetMarginSettings() MarginSettings
	// QueryMarginAccount(ctx context.Context) (*binance.MarginAccount, error)
}

type MarginSettings struct {
	IsMargin             bool
	IsIsolatedMargin     bool
	IsolatedMarginSymbol string
}

func (e MarginSettings) GetMarginSettings() MarginSettings {
	return e
}

func (e *MarginSettings) UseMargin() {
	e.IsMargin = true
}

func (e *MarginSettings) UseIsolatedMargin(symbol string) {
	e.IsMargin = true
	e.IsIsolatedMargin = true
	e.IsolatedMarginSymbol = symbol
}

// MarginAccount is for the cross margin account
type MarginAccount struct {
	BorrowEnabled       bool              `json:"borrowEnabled"`
	MarginLevel         fixedpoint.Value  `json:"marginLevel"`
	TotalAssetOfBTC     fixedpoint.Value  `json:"totalAssetOfBtc"`
	TotalLiabilityOfBTC fixedpoint.Value  `json:"totalLiabilityOfBtc"`
	TotalNetAssetOfBTC  fixedpoint.Value  `json:"totalNetAssetOfBtc"`
	TradeEnabled        bool              `json:"tradeEnabled"`
	TransferEnabled     bool              `json:"transferEnabled"`
	UserAssets          []MarginUserAsset `json:"userAssets"`
}

// MarginUserAsset define user assets of margin account
type MarginUserAsset struct {
	Asset    string           `json:"asset"`
	Borrowed fixedpoint.Value `json:"borrowed"`
	Free     fixedpoint.Value `json:"free"`
	Interest fixedpoint.Value `json:"interest"`
	Locked   fixedpoint.Value `json:"locked"`
	NetAsset fixedpoint.Value `json:"netAsset"`
}

// IsolatedMarginAccount defines isolated user assets of margin account
type IsolatedMarginAccount struct {
	TotalAssetOfBTC     fixedpoint.Value      `json:"totalAssetOfBtc"`
	TotalLiabilityOfBTC fixedpoint.Value      `json:"totalLiabilityOfBtc"`
	TotalNetAssetOfBTC  fixedpoint.Value      `json:"totalNetAssetOfBtc"`
	Assets              []IsolatedMarginAsset `json:"assets"`
}

// IsolatedMarginAsset defines isolated margin asset information, like margin level, liquidation price... etc
type IsolatedMarginAsset struct {
	Symbol     string            `json:"symbol"`
	QuoteAsset IsolatedUserAsset `json:"quoteAsset"`
	BaseAsset  IsolatedUserAsset `json:"baseAsset"`

	IsolatedCreated   bool             `json:"isolatedCreated"`
	MarginLevel       fixedpoint.Value `json:"marginLevel"`
	MarginLevelStatus string           `json:"marginLevelStatus"`

	MarginRatio    fixedpoint.Value `json:"marginRatio"`
	IndexPrice     fixedpoint.Value `json:"indexPrice"`
	LiquidatePrice fixedpoint.Value `json:"liquidatePrice"`
	LiquidateRate  fixedpoint.Value `json:"liquidateRate"`

	TradeEnabled bool `json:"tradeEnabled"`
}

// IsolatedUserAsset defines isolated user assets of the margin account
type IsolatedUserAsset struct {
	Asset         string           `json:"asset"`
	Borrowed      fixedpoint.Value `json:"borrowed"`
	Free          fixedpoint.Value `json:"free"`
	Interest      fixedpoint.Value `json:"interest"`
	Locked        fixedpoint.Value `json:"locked"`
	NetAsset      fixedpoint.Value `json:"netAsset"`
	NetAssetOfBtc fixedpoint.Value `json:"netAssetOfBtc"`

	BorrowEnabled bool             `json:"borrowEnabled"`
	RepayEnabled  bool             `json:"repayEnabled"`
	TotalAsset    fixedpoint.Value `json:"totalAsset"`
}

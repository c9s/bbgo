package types

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

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

type MarginExchange interface {
	UseMargin()
	UseIsolatedMargin(symbol string)
	GetMarginSettings() MarginSettings
}

// MarginBorrowRepay provides repay and borrow actions of an crypto exchange
type MarginBorrowRepay interface {
	RepayMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error
	BorrowMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error
	QueryMarginAssetMaxBorrowable(ctx context.Context, asset string) (amount fixedpoint.Value, err error)
}

type MarginInterest struct {
	Asset          string           `json:"asset" db:"asset"`
	Principle      fixedpoint.Value `json:"principle" db:"principle"`
	Interest       fixedpoint.Value `json:"interest" db:"interest"`
	InterestRate   fixedpoint.Value `json:"interestRate" db:"interest_rate"`
	IsolatedSymbol string           `json:"isolatedSymbol" db:"isolated_symbol"`
	Time           Time             `json:"time" db:"time"`
}

type MarginLoanRecord struct {
	TransactionID  uint64           `json:"transactionID" db:"transaction_id"`
	Asset          string           `json:"asset" db:"asset"`
	Principle      fixedpoint.Value `json:"principle" db:"principle"`
	Time           Time             `json:"time" db:"time"`
	IsolatedSymbol string           `json:"isolatedSymbol" db:"isolated_symbol"`
}

type MarginRepayRecord struct {
	TransactionID  uint64           `json:"transactionID" db:"transaction_id"`
	Asset          string           `json:"asset" db:"asset"`
	Principle      fixedpoint.Value `json:"principle" db:"principle"`
	Time           Time             `json:"time" db:"time"`
	IsolatedSymbol string           `json:"isolatedSymbol" db:"isolated_symbol"`
}

type MarginLiquidationRecord struct {
	AveragePrice     fixedpoint.Value `json:"avgPrice"`
	ExecutedQuantity fixedpoint.Value `json:"executedQty"`
	OrderId          uint64           `json:"orderId"`
	Price            fixedpoint.Value `json:"price"`
	Qty              fixedpoint.Value `json:"qty"`
	Side             SideType         `json:"side"`
	Symbol           string           `json:"symbol"`
	TimeInForce      TimeInForce      `json:"timeInForce"`
	IsIsolated       bool             `json:"isIsolated"`
	UpdatedTime      Time             `json:"updatedTime"`
}

// MarginHistory provides the service of querying loan history and repay history
type MarginHistory interface {
	QueryLoanHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]MarginLoanRecord, error)
	QueryRepayHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]MarginRepayRecord, error)
	QueryLiquidationHistory(ctx context.Context, startTime, endTime *time.Time) ([]MarginLiquidationRecord, error)
	QueryInterestHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]MarginInterest, error)
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
	TotalAssetOfBTC     fixedpoint.Value       `json:"totalAssetOfBtc"`
	TotalLiabilityOfBTC fixedpoint.Value       `json:"totalLiabilityOfBtc"`
	TotalNetAssetOfBTC  fixedpoint.Value       `json:"totalNetAssetOfBtc"`
	Assets              IsolatedMarginAssetMap `json:"assets"`
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

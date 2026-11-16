package types

import (
	"context"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type MarginType string

const (
	MarginTypeSpot           MarginType = "spot"
	MarginTypeCrossMargin    MarginType = "cross_margin"
	MarginTypeIsolatedMargin MarginType = "isolated_margin"
)

type MarginExchange interface {
	UseMargin()
	UseIsolatedMargin(symbol string)
	GetMarginSettings() MarginSettings
}

// MarginBorrowRepayService provides repay and borrow actions of an crypto exchange
type MarginBorrowRepayService interface {
	RepayMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error
	BorrowMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error
	QueryMarginAssetMaxBorrowable(ctx context.Context, asset string) (amount fixedpoint.Value, err error)
}

type MarginAssetInfo struct {
	Asset        string
	IsBorrowable bool
	MinBorrow    fixedpoint.Value
	MinRepay     fixedpoint.Value
}

type MarginAssetInfoService interface {
	QueryMarginAssetInfo(ctx context.Context, asset string) (MarginAssetInfo, error)

	// QueryMarginAssetInfos returns the entries of the given assets, fetched in bulk.
	// An asset the exchange does not list is absent from the result.
	QueryMarginAssetInfos(ctx context.Context, assets []string) (map[string]MarginAssetInfo, error)
}

// MarginInterest is an interest charge, stored as the exchange returns it.
// Binance is the reference format: other exchanges map their records to its fields.
type MarginInterest struct {
	GID      uint64       `json:"gid" db:"gid"`
	Exchange ExchangeName `json:"exchange" db:"exchange"`

	// Session is the bbgo session the row was synced for, stamped by the sync.
	// It is '' on the rows synced before the session was stored.
	Session string `json:"session" db:"session"`

	// TxID is the exchange's ID of the charge (Binance txId). It is nil when
	// the exchange has none and on the rows synced before it was stored.
	TxID *uint64 `json:"txID,omitempty" db:"tx_id"`

	// Asset is the asset the interest is paid in, e.g. BNB when Binance converts it
	Asset string `json:"asset" db:"asset"`

	// RawAsset is the borrowed asset (Binance rawAsset), '' when the exchange does not return it
	RawAsset string `json:"rawAsset" db:"raw_asset"`

	// InterestType is the exchange's type of the charge (Binance type), e.g. PERIODIC or ON_BORROW_CONVERTED
	InterestType string `json:"interestType" db:"interest_type"`

	Principal      fixedpoint.Value `json:"principal" db:"principal"`
	Interest       fixedpoint.Value `json:"interest" db:"interest"`
	InterestRate   fixedpoint.Value `json:"interestRate" db:"interest_rate"`
	IsolatedSymbol string           `json:"isolatedSymbol" db:"isolated_symbol"`
	Time           Time             `json:"time" db:"time"`
}

// Key identifies the charge: its TxID, or for a row without one, its asset,
// isolated symbol and time. Several charges can share an hour and an asset,
// so rows with a TxID are never told apart by their columns.
func (m MarginInterest) Key() string {
	if m.TxID != nil {
		return "tx:" + strconv.FormatUint(*m.TxID, 10)
	}

	return m.Asset + "|" + m.IsolatedSymbol + "|" + strconv.FormatInt(m.Time.UnixMilli(), 10)
}

// MarginInterestValuation is a margin interest charge valued in USDT at the time
// it was charged. It is derived from a margin_interests row and the 1h klines,
// and can be recomputed from them at any time.
type MarginInterestValuation struct {
	GID uint64 `json:"gid" db:"gid"`

	// InterestGID is the gid of the margin_interests row
	InterestGID uint64 `json:"interestGID" db:"interest_gid"`

	Exchange       ExchangeName `json:"exchange" db:"exchange"`
	Session        string       `json:"session" db:"session"`
	Time           Time         `json:"time" db:"time"`
	IsolatedSymbol string       `json:"isolatedSymbol" db:"isolated_symbol"`

	// PaidAsset is the asset the interest was paid in, e.g. BNB when Binance converts it
	PaidAsset string `json:"paidAsset" db:"paid_asset"`

	// BorrowedAsset is the asset the interest was charged on. It is '' when
	// unknown: a converted row without rawAsset, e.g. on isolated margin.
	BorrowedAsset string `json:"borrowedAsset" db:"borrowed_asset"`

	// InterestType is the exchange's type of the charge, copied from margin_interests,
	// e.g. PERIODIC, ON_BORROW or PERIODIC_CONVERTED on Binance
	InterestType string `json:"interestType" db:"interest_type"`

	// Converted is true when the interest was paid in another asset than the borrowed one
	Converted bool `json:"converted" db:"converted"`

	// Principal is the borrowed amount the interest was charged on, in BorrowedAsset units
	Principal fixedpoint.Value `json:"principal" db:"principal"`

	// InterestRate is the exchange's rate of the charge, a daily rate on Binance
	InterestRate fixedpoint.Value `json:"interestRate" db:"interest_rate"`

	// Interest is in PaidAsset units
	Interest fixedpoint.Value `json:"interest" db:"interest"`

	// BorrowedInterest is in BorrowedAsset units: Interest, or on a converted row
	// principal x daily rate / 24, an estimate. nil when BorrowedAsset is unknown.
	BorrowedInterest *fixedpoint.Value `json:"borrowedInterest,omitempty" db:"borrowed_interest"`

	// PriceSymbol is the market PriceInUSDT comes from, e.g. BNBUSDT, or USDTTRY
	// for an inverse pair; '' for USDT or when the asset has no USDT market
	PriceSymbol string `json:"priceSymbol" db:"price_symbol"`

	// PriceKLineStart is the start of the 1h kline whose close is a price of the
	// row, the hour before the charge hour. Zero when no price came from a kline.
	PriceKLineStart Time `json:"priceKLineStart" db:"price_kline_start"`

	// PriceInUSDT is the USDT price of PaidAsset; nil when not priced (yet)
	PriceInUSDT *fixedpoint.Value `json:"priceInUSDT,omitempty" db:"price_in_usdt"`

	// InterestUSDT is Interest x PriceInUSDT, what the charge cost; nil when not priced (yet)
	InterestUSDT *fixedpoint.Value `json:"interestUSDT,omitempty" db:"interest_usdt"`

	// BorrowedPriceSymbol is the market BorrowedPriceInUSDT comes from, like PriceSymbol
	BorrowedPriceSymbol string `json:"borrowedPriceSymbol" db:"borrowed_price_symbol"`

	// BorrowedPriceInUSDT is the USDT price of BorrowedAsset, PriceInUSDT on a
	// row that was not converted; nil when not priced (yet) or BorrowedAsset is unknown
	BorrowedPriceInUSDT *fixedpoint.Value `json:"borrowedPriceInUSDT,omitempty" db:"borrowed_price_in_usdt"`

	// BorrowedInterestUSDT is BorrowedInterest x BorrowedPriceInUSDT, what the
	// charge would have cost in the borrowed asset; nil when not priced (yet)
	BorrowedInterestUSDT *fixedpoint.Value `json:"borrowedInterestUSDT,omitempty" db:"borrowed_interest_usdt"`

	// PrincipalUSDT is Principal x BorrowedPriceInUSDT; nil when not priced (yet)
	PrincipalUSDT *fixedpoint.Value `json:"principalUSDT,omitempty" db:"principal_usdt"`

	// PriceStatus tells whether every USDT value of the row is set, and why not
	PriceStatus MarginInterestPriceStatus `json:"priceStatus" db:"price_status"`

	UpdatedTime Time `json:"updatedTime" db:"updated_at"`
}

// MarginInterestPriceStatus tells whether the USDT values of a
// MarginInterestValuation are set. When several apply, the first listed wins.
type MarginInterestPriceStatus string

const (
	// MarginInterestKLineMissing: a 1h kline the row needs is not stored yet.
	// The row is valued again by the next sync.
	MarginInterestKLineMissing MarginInterestPriceStatus = "kline_missing"

	// MarginInterestNoMarket: the paid or the borrowed asset has neither a USDT
	// nor an inverse market. Final until the table is rebuilt.
	MarginInterestNoMarket MarginInterestPriceStatus = "no_market"

	// MarginInterestUnknownBorrowed: the paid asset is priced, but the borrowed
	// asset is unknown (a converted row without rawAsset), so the borrowed values are nil. Final.
	MarginInterestUnknownBorrowed MarginInterestPriceStatus = "unknown_borrowed"

	// MarginInterestPriced: every USDT value is set
	MarginInterestPriced MarginInterestPriceStatus = "priced"
)

type MarginLoan struct {
	GID            uint64           `json:"gid" db:"gid"`
	Exchange       ExchangeName     `json:"exchange" db:"exchange"`
	TransactionID  uint64           `json:"transactionID" db:"transaction_id"`
	Asset          string           `json:"asset" db:"asset"`
	Principle      fixedpoint.Value `json:"principle" db:"principle"`
	Time           Time             `json:"time" db:"time"`
	IsolatedSymbol string           `json:"isolatedSymbol" db:"isolated_symbol"`
}

type MarginRepay struct {
	GID            uint64           `json:"gid" db:"gid"`
	Exchange       ExchangeName     `json:"exchange" db:"exchange"`
	TransactionID  uint64           `json:"transactionID" db:"transaction_id"`
	Asset          string           `json:"asset" db:"asset"`
	Principle      fixedpoint.Value `json:"principle" db:"principle"`
	Time           Time             `json:"time" db:"time"`
	IsolatedSymbol string           `json:"isolatedSymbol" db:"isolated_symbol"`
}

type MarginLiquidation struct {
	GID              uint64           `json:"gid" db:"gid"`
	Exchange         ExchangeName     `json:"exchange" db:"exchange"`
	AveragePrice     fixedpoint.Value `json:"averagePrice" db:"average_price"`
	ExecutedQuantity fixedpoint.Value `json:"executedQuantity" db:"executed_quantity"`
	OrderID          uint64           `json:"orderID" db:"order_id"`
	Price            fixedpoint.Value `json:"price" db:"price"`
	Quantity         fixedpoint.Value `json:"quantity" db:"quantity"`
	Side             SideType         `json:"side" db:"side"`
	Symbol           string           `json:"symbol" db:"symbol"`
	TimeInForce      TimeInForce      `json:"timeInForce" db:"time_in_force"`
	IsIsolated       bool             `json:"isIsolated" db:"is_isolated"`
	UpdatedTime      Time             `json:"updatedTime" db:"time"`
}

// MarginHistoryService provides the service of querying loan history and repay history
type MarginHistoryService interface {
	QueryLoanHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]MarginLoan, error)
	QueryRepayHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]MarginRepay, error)
	QueryLiquidationHistory(ctx context.Context, startTime, endTime *time.Time) ([]MarginLiquidation, error)
	QueryInterestHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]MarginInterest, error)
}

type MarginSettings struct {
	IsMargin             bool
	IsIsolatedMargin     bool
	IsolatedMarginSymbol string
}

func (e *MarginSettings) GetMarginSettings() MarginSettings {
	return *e
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

type MarginNextHourlyInterestRate struct {
	Asset          string           `json:"asset"`
	HourlyRate     fixedpoint.Value `json:"hourlyRate"`
	AnnualizedRate fixedpoint.Value `json:"annualizedRate"`
}

type MarginNextHourlyInterestRateMap map[string]*MarginNextHourlyInterestRate

func (m MarginNextHourlyInterestRateMap) String() (out string) {
	for k, v := range m {
		out += k + " Hourly:" + v.HourlyRate.FormatPercentage(5) + " APY: " + v.AnnualizedRate.FormatPercentage(3) + "\n"
	}

	return out
}

package types

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const DateFormat = "2006-01-02"

//go:generate mapgen -type ExchangeName
type ExchangeName string

const (
	ExchangeMax      ExchangeName = "max"
	ExchangeBinance  ExchangeName = "binance"
	ExchangeOKEx     ExchangeName = "okex"
	ExchangeKucoin   ExchangeName = "kucoin"
	ExchangeBitget   ExchangeName = "bitget"
	ExchangeBacktest ExchangeName = "backtest"
	ExchangeBybit    ExchangeName = "bybit"
	ExchangeCoinBase ExchangeName = "coinbase"
	ExchangeBitfinex ExchangeName = "bitfinex"
)

var SupportedExchanges = map[ExchangeName]struct{}{
	ExchangeMax:      struct{}{},
	ExchangeBinance:  struct{}{},
	ExchangeOKEx:     struct{}{},
	ExchangeKucoin:   struct{}{},
	ExchangeBitget:   struct{}{},
	ExchangeBybit:    struct{}{},
	ExchangeCoinBase: struct{}{},
	ExchangeBitfinex: struct{}{},
	// note: we are not using "backtest"
}

func (n *ExchangeName) Value() (driver.Value, error) {
	return n.String(), nil
}

func (n *ExchangeName) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*n = ExchangeName(s)
	if !n.IsValid() {
		return fmt.Errorf("%s is an invalid exchange name", s)
	}

	return nil
}

func (n ExchangeName) IsValid() bool {
	if _, exist := SupportedExchanges[n]; exist {
		return true
	}

	return false
}

func (n ExchangeName) String() string {
	return string(n)
}

func ValidExchangeName(a string) (ExchangeName, error) {
	exName := ExchangeName(strings.ToLower(a))
	if !exName.IsValid() {
		return "", fmt.Errorf("invalid exchange name: %s", a)
	}

	return exName, nil
}

type Initializer interface {
	Initialize(ctx context.Context) error
}

type ExchangeMinimal interface {
	Name() ExchangeName
	PlatformFeeCurrency() string
}

//go:generate mockgen -destination=mocks/mock_exchange.go -package=mocks . Exchange
type Exchange interface {
	ExchangeMinimal
	ExchangeMarketDataService
	ExchangeAccountService
	ExchangeTradeService
}

//go:generate mockgen -destination=mocks/mock_exchange_extended.go -package=mocks . ExchangeExtended
type ExchangeExtended interface {
	Exchange
	ExchangeOrderQueryService
}

//go:generate mockgen -destination=mocks/mock_exchange_public.go -package=mocks . ExchangePublic
type ExchangePublic interface {
	ExchangeMinimal
	ExchangeMarketDataService
}

// ExchangeBasic is the new type for replacing the original Exchange interface
type ExchangeBasic = Exchange

// ExchangeOrderQueryService provides an interface for querying the order status via order ID or client order ID
//
//go:generate mockgen -destination=mocks/mock_exchange_order_query.go -package=mocks . ExchangeOrderQueryService
type ExchangeOrderQueryService interface {
	QueryOrder(ctx context.Context, q OrderQuery) (*Order, error)
	QueryOrderTrades(ctx context.Context, q OrderQuery) ([]Trade, error)
}

type ExchangeAccountService interface {
	QueryAccount(ctx context.Context) (*Account, error)

	QueryAccountBalances(ctx context.Context) (BalanceMap, error)
}

type ExchangeTradeService interface {
	SubmitOrder(ctx context.Context, order SubmitOrder) (createdOrder *Order, err error)

	QueryOpenOrders(ctx context.Context, symbol string) (orders []Order, err error)

	CancelOrders(ctx context.Context, orders ...Order) error
}

type ExchangeDefaultFeeRates interface {
	DefaultFeeRates() ExchangeFee
}

//go:generate mockgen -destination=mocks/mock_exchange_trade_history.go -package=mocks . ExchangeTradeHistoryService
type ExchangeTradeHistoryService interface {
	QueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) ([]Trade, error)
	QueryClosedOrders(
		ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64,
	) (orders []Order, err error)
}

type ExchangeMarketDataService interface {
	NewStream() Stream

	QueryMarkets(ctx context.Context) (MarketMap, error)

	QueryTicker(ctx context.Context, symbol string) (*Ticker, error)

	QueryTickers(ctx context.Context, symbol ...string) (map[string]Ticker, error)

	QueryKLines(ctx context.Context, symbol string, interval Interval, options KLineQueryOptions) ([]KLine, error)
}

type OTCExchange interface {
	RequestForQuote(
		ctx context.Context, symbol string, side SideType, quantity fixedpoint.Value,
	) (submitOrder SubmitOrder, err error)
}

type CustomIntervalProvider interface {
	SupportedInterval() map[Interval]int
	IsSupportedInterval(interval Interval) bool
}

type DepositHistoryService interface {
	QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []Deposit, err error)
}

type WithdrawHistoryService interface {
	QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []Withdraw, err error)
}

type ExchangeTransferHistoryService interface {
	DepositHistoryService
	WithdrawHistoryService
}

type ExchangeTransferService interface {
	QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []Deposit, err error)
	QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []Withdraw, err error)
}

type ExchangeWithdrawalService interface {
	Withdraw(
		ctx context.Context, asset string, amount fixedpoint.Value, address string, options *WithdrawalOptions,
	) error
}

type ExchangeRewardService interface {
	QueryRewards(ctx context.Context, startTime time.Time) ([]Reward, error)
}

type ExchangeRiskService interface {
	SetLeverage(ctx context.Context, symbol string, leverage int) error
	QueryPositionRisk(ctx context.Context, symbol ...string) ([]PositionRisk, error)
}

type TradeQueryOptions struct {
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int64
	LastTradeID uint64
}

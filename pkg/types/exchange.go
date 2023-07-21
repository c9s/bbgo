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

type ExchangeName string

func (n *ExchangeName) Value() (driver.Value, error) {
	return n.String(), nil
}

func (n *ExchangeName) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch s {
	case "max", "binance", "okex", "kucoin":
		*n = ExchangeName(s)
		return nil

	}

	return fmt.Errorf("unknown or unsupported exchange name: %s, valid names are: max, binance, okex, kucoin", s)
}

func (n ExchangeName) String() string {
	return string(n)
}

const (
	ExchangeMax      ExchangeName = "max"
	ExchangeBinance  ExchangeName = "binance"
	ExchangeOKEx     ExchangeName = "okex"
	ExchangeKucoin   ExchangeName = "kucoin"
	ExchangeBitget   ExchangeName = "bitget"
	ExchangeBacktest ExchangeName = "backtest"
	ExchangeBybit    ExchangeName = "bybit"
)

var SupportedExchanges = []ExchangeName{
	ExchangeMax,
	ExchangeBinance,
	ExchangeOKEx,
	ExchangeKucoin,
	ExchangeBitget,
	ExchangeBybit,
	// note: we are not using "backtest"
}

func ValidExchangeName(a string) (ExchangeName, error) {
	aa := strings.ToLower(a)
	for _, n := range SupportedExchanges {
		if string(n) == aa {
			return n, nil
		}
	}

	return "", fmt.Errorf("invalid exchange name: %s", a)
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

type ExchangeAmountFeeProtect interface {
	SetModifyOrderAmountForFee(ExchangeFee)
}

//go:generate mockgen -destination=mocks/mock_exchange_trade_history.go -package=mocks . ExchangeTradeHistoryService
type ExchangeTradeHistoryService interface {
	QueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) ([]Trade, error)
	QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []Order, err error)
}

type ExchangeMarketDataService interface {
	NewStream() Stream

	QueryMarkets(ctx context.Context) (MarketMap, error)

	QueryTicker(ctx context.Context, symbol string) (*Ticker, error)

	QueryTickers(ctx context.Context, symbol ...string) (map[string]Ticker, error)

	QueryKLines(ctx context.Context, symbol string, interval Interval, options KLineQueryOptions) ([]KLine, error)
}

type CustomIntervalProvider interface {
	SupportedInterval() map[Interval]int
	IsSupportedInterval(interval Interval) bool
}

type ExchangeTransferService interface {
	QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []Deposit, err error)
	QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []Withdraw, err error)
}

type ExchangeWithdrawalService interface {
	Withdraw(ctx context.Context, asset string, amount fixedpoint.Value, address string, options *WithdrawalOptions) error
}

type ExchangeRewardService interface {
	QueryRewards(ctx context.Context, startTime time.Time) ([]Reward, error)
}

type TradeQueryOptions struct {
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int64
	LastTradeID uint64
}

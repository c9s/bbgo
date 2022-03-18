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
	case "max", "binance", "ftx", "okex":
		*n = ExchangeName(s)
		return nil

	}

	return fmt.Errorf("unknown or unsupported exchange name: %s, valid names are: max, binance, ftx", s)
}

func (n ExchangeName) String() string {
	return string(n)
}

const (
	ExchangeMax      = ExchangeName("max")
	ExchangeBinance  = ExchangeName("binance")
	ExchangeFTX      = ExchangeName("ftx")
	ExchangeOKEx     = ExchangeName("okex")
	ExchangeKucoin   = ExchangeName("kucoin")
	ExchangeBacktest = ExchangeName("backtest")
)

var SupportedExchanges = []ExchangeName{"binance", "max", "ftx", "okex", "kucoin"}

func ValidExchangeName(a string) (ExchangeName, error) {
	switch strings.ToLower(a) {
	case "max":
		return ExchangeMax, nil
	case "binance", "bn":
		return ExchangeBinance, nil
	case "ftx":
		return ExchangeFTX, nil
	case "okex":
		return ExchangeOKEx, nil
	case "kucoin":
		return ExchangeKucoin, nil
	}

	return "", fmt.Errorf("invalid exchange name: %s", a)
}

type Exchange interface {
	Name() ExchangeName

	PlatformFeeCurrency() string

	ExchangeMarketDataService

	ExchangeTradeService
}

// ExchangeOrderQueryService provides an interface for querying the order status via order ID or client order ID
type ExchangeOrderQueryService interface {
	QueryOrder(ctx context.Context, q OrderQuery) (*Order, error)
}

type ExchangeTradeService interface {
	QueryAccount(ctx context.Context) (*Account, error)

	QueryAccountBalances(ctx context.Context) (BalanceMap, error)

	SubmitOrders(ctx context.Context, orders ...SubmitOrder) (createdOrders OrderSlice, err error)

	QueryOpenOrders(ctx context.Context, symbol string) (orders []Order, err error)

	CancelOrders(ctx context.Context, orders ...Order) error
}

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
	Withdrawal(ctx context.Context, asset string, amount fixedpoint.Value, address string, options *WithdrawalOptions) error
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

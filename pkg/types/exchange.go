package types

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const DateFormat = "2006-01-02"

type ExchangeName string

func (n *ExchangeName) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s) ; err != nil {
		return err
	}

	switch s {
	case "max", "binance", "ftx":
		*n = ExchangeName(s)
		return nil

	}

	return fmt.Errorf("unknown or unsupported exchange name: %s, valid names are: max, binance, ftx", s)
}

func (n ExchangeName) String() string {
	return string(n)
}

const (
	ExchangeMax     = ExchangeName("max")
	ExchangeBinance = ExchangeName("binance")
	ExchangeFTX     = ExchangeName("ftx")
)

func ValidExchangeName(a string) (ExchangeName, error) {
	switch strings.ToLower(a) {
	case "max":
		return ExchangeMax, nil
	case "binance", "bn":
		return ExchangeBinance, nil
	}

	return "", errors.New("invalid exchange name")
}

type Exchange interface {
	Name() ExchangeName

	PlatformFeeCurrency() string

	NewStream() Stream

	QueryMarkets(ctx context.Context) (MarketMap, error)

	QueryAccount(ctx context.Context) (*Account, error)

	QueryAccountBalances(ctx context.Context) (BalanceMap, error)

	QueryTicker(ctx context.Context, symbol string) (*Ticker, error)

	QueryTickers(ctx context.Context, symbol ...string) (map[string]Ticker, error)

	QueryKLines(ctx context.Context, symbol string, interval Interval, options KLineQueryOptions) ([]KLine, error)

	QueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) ([]Trade, error)

	QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []Deposit, err error)

	QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []Withdraw, err error)

	SubmitOrders(ctx context.Context, orders ...SubmitOrder) (createdOrders OrderSlice, err error)

	QueryOpenOrders(ctx context.Context, symbol string) (orders []Order, err error)

	QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []Order, err error)

	CancelOrders(ctx context.Context, orders ...Order) error
}

type ExchangeRewardService interface {
	QueryRewards(ctx context.Context, startTime time.Time) ([]Reward, error)
}

type TradeQueryOptions struct {
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int64
	LastTradeID int64
}

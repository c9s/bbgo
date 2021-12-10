package kucoin

import (
	"context"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

// OKB is the platform currency of OKEx, pre-allocate static string here
const KCS = "KCS"

var log = logrus.WithFields(logrus.Fields{
	"exchange": "kucoin",
})

type Exchange struct {
	key, secret, passphrase string
}

func (e *Exchange) NewStream() types.Stream {
	panic("implement me")
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	panic("implement me")
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	panic("implement me")
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	panic("implement me")
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	panic("implement me")
}

func New(key, secret, passphrase string) *Exchange {
	return &Exchange{
		key:        key,
		secret:     secret,
		passphrase: passphrase,
	}
}

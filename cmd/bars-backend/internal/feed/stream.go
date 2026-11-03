package feed

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/types"

	"github.com/c9s/bbgo/cmd/bars-backend/internal/config"
)

// Feed owns a single public exchange stream and the KLineManager/BarManager
// fed by its kline and trade events.
type Feed struct {
	cfg    *config.Config
	ex     types.Exchange
	stream types.Stream

	KLines *KLineManager
	Bars   *BarManager
}

// New builds a Feed for the exchange/symbols/intervals configured in cfg,
// subscribing to the kline channel for every (symbol, interval) pair and the
// market trade channel for every symbol, but does not connect yet.
func New(cfg *config.Config) (*Feed, error) {
	ex, err := newExchange(cfg.ExchangeName)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s exchange: %w", cfg.ExchangeName, err)
	}

	stream := ex.NewStream()
	stream.SetPublicOnly()

	f := &Feed{
		cfg:    cfg,
		ex:     ex,
		stream: stream,
		KLines: NewKLineManager(cfg.HistoryLimit),
		Bars:   NewBarManager(ex, cfg.HistoryLimit),
	}

	for _, symbol := range cfg.Symbols {
		for _, interval := range cfg.Intervals {
			stream.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: interval})
		}
		stream.Subscribe(types.MarketTradeChannel, symbol, types.SubscribeOptions{})
	}

	stream.OnKLine(f.KLines.handleKLine)
	stream.OnKLineClosed(f.KLines.handleKLine)
	stream.OnMarketTrade(f.Bars.handleTrade)

	return f, nil
}

// newExchange creates ex for the given exchange name, preferring credentials
// from the exchange's standard <NAME>_API_KEY/<NAME>_API_SECRET env vars
// (needed for Binance's historical-trades endpoint that backfills tick/volume
// bars) and falling back to an anonymous public-only instance otherwise; in
// that fallback, kline data still works but bar backfill will error until
// credentials are set.
func newExchange(name types.ExchangeName) (types.Exchange, error) {
	if exMinimal, err := exchange.NewWithEnvVarPrefix(name, ""); err == nil {
		if ex, ok := exMinimal.(types.Exchange); ok {
			return ex, nil
		}
	}

	return exchange.NewPublic(name)
}

// Start backfills every tracked (symbol, interval) kline series via REST and
// then connects the live stream.
func (f *Feed) Start(ctx context.Context) error {
	for _, symbol := range f.cfg.Symbols {
		for _, interval := range f.cfg.Intervals {
			kLines, err := f.ex.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{Limit: f.cfg.HistoryLimit})
			if err != nil {
				return fmt.Errorf("failed to backfill klines for %s %s: %w", symbol, interval, err)
			}
			f.KLines.SetHistory(symbol, interval, kLines)
		}
	}

	return f.stream.Connect(ctx)
}

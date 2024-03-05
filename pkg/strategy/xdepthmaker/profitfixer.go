package xdepthmaker

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitFixerConfig struct {
	TradesSince types.Time `json:"tradesSince,omitempty"`
}

// ProfitFixer implements a trade history based profit fixer
type ProfitFixer struct {
	market types.Market

	sessions map[string]types.ExchangeTradeHistoryService
}

func NewProfitFixer(market types.Market) *ProfitFixer {
	return &ProfitFixer{
		market:   market,
		sessions: make(map[string]types.ExchangeTradeHistoryService),
	}
}

func (f *ProfitFixer) AddExchange(sessionName string, service types.ExchangeTradeHistoryService) {
	f.sessions[sessionName] = service
}

func (f *ProfitFixer) batchQueryTrades(
	ctx context.Context,
	service types.ExchangeTradeHistoryService,
	symbol string,
	since time.Time,
) ([]types.Trade, error) {

	now := time.Now()
	q := &batch.TradeBatchQuery{ExchangeTradeHistoryService: service}
	trades, err := q.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
		StartTime: &since,
		EndTime:   &now,
	})

	return trades, err
}

func (f *ProfitFixer) Fix(ctx context.Context, since time.Time) (*types.ProfitStats, error) {
	stats := types.NewProfitStats(f.market)

	var mu sync.Mutex
	var allTrades = make([]types.Trade, 0, 1000)

	g, subCtx := errgroup.WithContext(ctx)
	for _, service := range f.sessions {
		g.Go(func() error {
			trades, err := f.batchQueryTrades(subCtx, service, f.market.Symbol, since)
			if err != nil {
				log.WithError(err).Errorf("unable to batch query trades for fixer")
				return err
			}

			mu.Lock()
			allTrades = append(allTrades, trades...)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	allTrades = types.SortTradesAscending(allTrades)
	for _, trade := range allTrades {
		stats.AddTrade(trade)
	}

	return stats, nil
}

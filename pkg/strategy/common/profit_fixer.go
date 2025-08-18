package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

// ProfitFixerConfig is used for fixing profitStats and position by re-playing the trade history
type ProfitFixerConfig struct {
	TradesSince types.Time `json:"tradesSince,omitempty"`
}

// ProfitFixer implements a trade-history-based profit fixer
type ProfitFixer struct {
	sessions map[string]types.ExchangeTradeHistoryService

	core.ConverterManager
}

func NewProfitFixer() *ProfitFixer {
	return &ProfitFixer{
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
	since, until time.Time,
) (chan types.Trade, chan error) {
	q := &batch.TradeBatchQuery{ExchangeTradeHistoryService: service}
	return q.Query(ctx, symbol, &types.TradeQueryOptions{
		StartTime: &since,
		EndTime:   &until,
	})
}

func (f *ProfitFixer) aggregateAllTrades(ctx context.Context, symbol string, since, until time.Time) ([]types.Trade, error) {
	var mu sync.Mutex
	var allTrades = make([]types.Trade, 0, 1000)

	g, subCtx := errgroup.WithContext(ctx)
	for n, s := range f.sessions {
		// allocate a copy of the iteration variables
		sessionName := n
		service := s
		g.Go(func() error {
			log.Infof("batch querying %s trade history from %s since %s until %s", symbol, sessionName, since.String(), until.String())
			tradeC, errC := f.batchQueryTrades(subCtx, service, symbol, since, until)

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()

				case err := <-errC:
					return err

				case trade, ok := <-tradeC:
					if !ok {
						return nil
					}

					mu.Lock()
					allTrades = append(allTrades, trade)
					mu.Unlock()
				}
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	mu.Lock()
	allTrades = types.SortTradesAscending(allTrades)
	mu.Unlock()

	return allTrades, nil
}

func (f *ProfitFixer) Fix(
	ctx context.Context, symbol string, since, until time.Time, stats *types.ProfitStats, position *types.Position,
) error {
	log.Infof("starting profitFixer with time range %s <=> %s", since, until)
	allTrades, err := f.aggregateAllTrades(ctx, symbol, since, until)
	if err != nil {
		return err
	}

	return f.FixFromTrades(allTrades, stats, position)
}

func (f *ProfitFixer) FixFromTrades(allTrades []types.Trade, stats *types.ProfitStats, position *types.Position) error {
	for _, trade := range allTrades {
		trade = f.ConverterManager.ConvertTrade(trade)

		profit, netProfit, madeProfit := position.AddTrade(trade)
		if madeProfit {
			p := position.NewProfit(trade, profit, netProfit)
			stats.AddProfit(p)
		}
	}

	log.Infof("profitFixer fix finished: profitStats and position are updated from %d trades", len(allTrades))
	return nil
}

type ProfitFixerBundle struct {
	ProfitFixerConfig *ProfitFixerConfig `json:"profitFixer,omitempty"`
}

func (f *ProfitFixerBundle) Fix(
	ctx context.Context,
	symbol string,
	position *types.Position,
	profitStats *types.ProfitStats,
	sessions ...*bbgo.ExchangeSession,
) error {
	bbgo.Notify("Fixing %s profitStats and position...", symbol)

	log.Infof("profitFixer is enabled, checking checkpoint: %+v", f.ProfitFixerConfig.TradesSince)

	if f.ProfitFixerConfig.TradesSince.Time().IsZero() {
		return fmt.Errorf("tradesSince time can not be zero")
	}

	fixer := NewProfitFixer()
	for _, session := range sessions {
		if ss, ok := session.Exchange.(types.ExchangeTradeHistoryService); ok {
			log.Infof("adding makerSession %s to profitFixer", session.Name)
			fixer.AddExchange(session.Name, ss)
		}
	}

	return fixer.Fix(ctx,
		symbol,
		f.ProfitFixerConfig.TradesSince.Time(),
		time.Now(),
		profitStats,
		position)
}

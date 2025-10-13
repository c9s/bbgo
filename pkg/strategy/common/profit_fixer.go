package common

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// ProfitFixerConfig is used for fixing profitStats and position by re-playing the trade history
type ProfitFixerConfig struct {
	TradesSince types.Time `json:"tradesSince,omitempty"`
	Patch       string     `json:"patch,omitempty"`
}

func (c *ProfitFixerConfig) Equal(other *ProfitFixerConfig) bool {
	if c == nil && other == nil {
		return true
	}
	if c == nil || other == nil {
		return false
	}
	rt := reflect.TypeOf(*c)
	rv1 := reflect.ValueOf(*c)
	rv2 := reflect.ValueOf(*other)

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.Type.Kind() == reflect.Pointer {
			// skip pointer field comparison
			log.Warnf("skip pointer field comparison for profit fixer config: %s", f.Name)
			continue
		}
		v1 := rv1.FieldByName(f.Name).Interface()
		v2 := rv2.FieldByName(f.Name).Interface()
		if v1 != v2 {
			return false
		}
	}
	return true
}

// ProfitFixer implements a trade-history-based profit fixer
type ProfitFixer struct {
	sessions map[string]types.ExchangeTradeHistoryService
	// (token, date) -> price
	tokenFeePrices map[tokenFeeKey]fixedpoint.Value

	core.ConverterManager
}

type tokenFeeKey struct {
	token        string
	exchangeName types.ExchangeName
	date         string
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

				case trade, ok := <-tradeC:
					if !ok {
						err := <-errC
						return err
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

func (f *ProfitFixer) buildTokenFeeMap(ctx context.Context, trades []types.Trade, since, until time.Time) error {
	// initialize tokenFeePrices map
	f.tokenFeePrices = make(map[tokenFeeKey]fixedpoint.Value)

	if len(trades) == 0 {
		return nil
	}

	// token -> symbol, exchangeName
	tokens := make(map[tokenFeeKey]struct{})
	// exchangeName -> markets
	markets := make(map[types.ExchangeName]types.MarketMap)
	// exchangeName -> ExchangePublic: query required data by trade.Exchange
	exchanges := make(map[types.ExchangeName]types.Exchange)
	for sessionName, service := range f.sessions {
		if ex, ok := service.(types.Exchange); ok {
			exchanges[ex.Name()] = ex
			mm, err := ex.QueryMarkets(ctx)
			if err == nil {
				markets[ex.Name()] = mm
			}
		} else {
			log.Warnf("session does not implement types.Exchange: %s", sessionName)
		}
	}

	// all exchanges do not implement ExchangePublic, can not build token fee map
	if len(exchanges) == 0 {
		return nil
	}

	var quoteCurrency string // quote currency is assumed to be the same for all trades
	for _, trade := range trades {
		// skip trade if fee currency is USD*
		if strings.HasPrefix(trade.FeeCurrency, "USD") {
			continue
		}
		// skip trade if we do not have market info
		if _, ok := markets[trade.Exchange]; !ok {
			continue
		}
		market := markets[trade.Exchange][trade.Symbol]

		// skip trade if fee currency is base currency
		// since position.AddTrade already handle base currency fee
		if trade.FeeCurrency == market.BaseCurrency {
			continue
		}
		// sanity check: all quote currency should be the same
		if quoteCurrency != "" && quoteCurrency != market.QuoteCurrency {
			return fmt.Errorf("quote currency mismatch: %s != %s", quoteCurrency, market.QuoteCurrency)
		}
		quoteCurrency = market.QuoteCurrency
		tokens[tokenFeeKey{
			token:        trade.FeeCurrency,
			exchangeName: trade.Exchange,
			date:         "", // date is a dummy here
		}] = struct{}{}
	}
	// no quote currency found if:
	// - all fees are USD*, or
	// - all fees are base currency
	// no need to build token fee map in this case
	if quoteCurrency == "" {
		return nil
	}
	startTime := since.Truncate(24 * time.Hour).Add(-24 * time.Hour)
	endTime := until.Truncate(24 * time.Hour)
	for info := range tokens {
		ex, ok := exchanges[info.exchangeName]
		if !ok {
			log.Warnf("can not build token fee on exchange %s: %s", info.exchangeName, info.token)
			continue
		}
		if err := func() error {
			query := &batch.KLineBatchQuery{Exchange: ex}
			kLineC, errC := query.Query(ctx, info.token+quoteCurrency, types.Interval1d, startTime, endTime)
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case kline, ok := <-kLineC:
					if !ok {
						err := <-errC
						return err
					}
					// the date in tokenFeeKey is the next day of the kline date
					// which means the token fee for the next day is calculated by the previous day's closing price
					f.tokenFeePrices[tokenFeeKey{
						token:        info.token,
						exchangeName: info.exchangeName,
						date:         kline.StartTime.Time().Add(24 * time.Hour).Format(time.DateOnly),
					}] = kline.Close
				}
			}
		}(); err != nil {
			return err
		}
	}
	return nil
}

func (f *ProfitFixer) Fix(
	ctx context.Context, symbol string, since, until time.Time, stats *types.ProfitStats, position *types.Position,
) error {
	log.Infof("starting profitFixer with time range %s <=> %s", since, until)
	allTrades, err := f.aggregateAllTrades(ctx, symbol, since, until)
	if err != nil {
		return err
	}
	if len(allTrades) == 0 {
		log.Warnf("[%s] no trades found between %s and %s, skip profit fixing", symbol, since.String(), until.String())
		return nil
	}
	err = f.buildTokenFeeMap(ctx, allTrades, since, until)
	if err != nil {
		return err
	}
	return f.fixFromTrades(allTrades, stats, position)
}

func (f *ProfitFixer) fixFromTrades(allTrades []types.Trade, stats *types.ProfitStats, position *types.Position) error {
	for _, trade := range allTrades {
		trade = f.ConverterManager.ConvertTrade(trade)
		// set fee average cost
		if feePrice, ok := f.tokenFeePrices[tokenFeeKey{
			token:        trade.FeeCurrency,
			exchangeName: trade.Exchange,
			date:         trade.Time.Time().Format(time.DateOnly),
		}]; ok {
			position.SetFeeAverageCost(trade.FeeCurrency, feePrice)
		}
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

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
	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

// ProfitFixerConfig is used for fixing profitStats and position by re-playing the trade history
type ProfitFixerConfig struct {
	TradesSince       types.Time `json:"tradesSince,omitempty"`
	Patch             string     `json:"patch,omitempty"`
	UseDatabaseTrades bool       `json:"useDatabaseTrades,omitempty"`
	ProfitCurrency    string     `json:"profitCurrency,omitempty"` // the currency to calculate profit in
	FeeCurrencies     []string   `json:"feeCurrencies,omitempty"`  // list of fee currencies to consider for fee price conversion
}

func NewProfitFixer(config ProfitFixerConfig, environment *bbgo.Environment) *ProfitFixer {
	fixer := newProfitFixer(environment)
	fixer.profitCurrency = config.ProfitCurrency
	if config.UseDatabaseTrades {
		fixer.queryTrades = fixer.queryTradesFromDB
	} else {
		fixer.queryTrades = fixer.queryTradesRestful
	}
	for _, feeCurrency := range config.FeeCurrencies {
		fixer.addFeeCurrency(feeCurrency)
	}
	return fixer
}

func (c ProfitFixerConfig) Equal(other ProfitFixerConfig) bool {
	return c.TradesSince.Equal(other.TradesSince.Time()) && c.Patch == other.Patch && c.UseDatabaseTrades == other.UseDatabaseTrades && c.ProfitCurrency == other.ProfitCurrency
}

// ProfitFixer implements a trade-history-based profit fixer
type ProfitFixer struct {
	sessions       map[string]types.ExchangeTradeHistoryService
	profitCurrency string
	feeCurrencies  map[string]struct{}

	core.ConverterManager
	*bbgo.Environment

	queryTrades func(ctx context.Context, symbol string, since, until time.Time) ([]types.Trade, error)
}

type tokenFeeKey struct {
	token        string
	exchangeName types.ExchangeName
	date         string
}

func newProfitFixer(environment *bbgo.Environment) *ProfitFixer {
	return &ProfitFixer{
		sessions:    make(map[string]types.ExchangeTradeHistoryService),
		Environment: environment,
	}
}

func (f *ProfitFixer) SetConverter(converter *core.ConverterManager) {
	f.ConverterManager = *converter
}

func (f *ProfitFixer) addFeeCurrency(feeCurrency string) {
	if f.feeCurrencies == nil {
		f.feeCurrencies = make(map[string]struct{})
	}
	f.feeCurrencies[feeCurrency] = struct{}{}
}

func (f *ProfitFixer) getFeeCurrencies() []string {
	var feeCurrencies []string
	for feeCurrency := range f.feeCurrencies {
		feeCurrencies = append(feeCurrencies, feeCurrency)
	}
	return feeCurrencies
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

func (f *ProfitFixer) queryTradesRestful(ctx context.Context, symbol string, since, until time.Time) ([]types.Trade, error) {
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

func (f *ProfitFixer) buildTokenFeeDatePrices(ctx context.Context, trades []types.Trade, since, until time.Time) (map[tokenFeeKey]fixedpoint.Value, error) {
	// initialize tokenFeePrices map
	tokenFeePrices := make(map[tokenFeeKey]fixedpoint.Value)

	if len(trades) == 0 {
		return tokenFeePrices, nil
	}

	feeCurrencies := f.getFeeCurrencies()
	if len(feeCurrencies) == 0 {
		return tokenFeePrices, nil
	}

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
		return tokenFeePrices, nil
	}

	startTime := since.Truncate(24 * time.Hour).Add(-24 * time.Hour)
	endTime := until.Truncate(24 * time.Hour)
	for exName, exchange := range exchanges {
		markets, ok := markets[exName]
		if !ok {
			continue
		}
		priceSolver := pricesolver.NewSimplePriceResolver(markets)
		for _, token := range feeCurrencies {
			// fee token is the profit currency, no conversion needed
			if token == f.profitCurrency {
				continue
			}
			// find the trading symbol
			var symbol string
			if m, ok := markets.FindPair(token, f.profitCurrency); ok {
				symbol = m.Symbol
			} else if m, ok := markets.FindPair(f.profitCurrency, token); ok {
				symbol = m.Symbol
			}
			if symbol == "" {
				// token is not tradable on this exchange, skip
				continue
			}

			if klines, err := queryKLines(
				ctx, exchange, symbol, types.Interval1d, startTime, endTime,
			); err == nil {
				for _, kline := range klines {
					// we still use the last day's closing price to calculate the USD fee price.
					// This is different from the actual fee calculation while trading which use the close price of the last minute.
					// But assuming the price of USD stablecoins do not fluctuate much within a day,
					// this should be acceptable.
					priceSolver.Update(symbol, kline.Close)
					date := kline.StartTime.Time().Add(24 * time.Hour).Format(time.DateOnly)
					if feePrice, ok := priceSolver.ResolvePrice(token, f.profitCurrency); ok {
						tokenFeePrices[tokenFeeKey{
							token:        token,
							exchangeName: exName,
							date:         date,
						}] = feePrice
					} else {
						log.Warnf(
							"cannot resolve fee price for %s on %s at date %s",
							token, exName, date,
						)
					}
				}
			}
		}
	}
	return tokenFeePrices, nil
}

func queryKLines(ctx context.Context, ex types.Exchange, symbol string, interval types.Interval, startTime, endTime time.Time) ([]types.KLine, error) {
	query := &batch.KLineBatchQuery{Exchange: ex}
	kLineC, errC := query.Query(ctx, symbol, interval, startTime, endTime)

	var kLines []types.KLine
	var err error
	done := false
	for !done {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case kline, ok := <-kLineC:
			if !ok {
				err = <-errC
				done = true
			}
			kLines = append(kLines, kline)
		}
	}
	return kLines, err
}

func (f *ProfitFixer) Fix(
	ctx context.Context, symbol string, since, until time.Time, stats *types.ProfitStats, position *types.Position,
) error {
	if f.profitCurrency == "" {
		return fmt.Errorf("quote currency is empty for profit fixing")
	}
	log.Infof("start profit fixing with time range %s <=> %s", since, until)
	allTrades, err := f.queryTrades(ctx, symbol, since, until)
	if err != nil {
		return err
	}
	if len(allTrades) == 0 {
		log.Warnf("[%s] no trades found between %s and %s, skip profit fixing", symbol, since.String(), until.String())
		return nil
	}
	fm, err := f.buildTokenFeeDatePrices(ctx, allTrades, since, until)
	if err != nil {
		return err
	}
	return f.fixFromTrades(allTrades, fm, stats, position)
}

func (f *ProfitFixer) fixFromTrades(
	allTrades []types.Trade, tokenFeePrices map[tokenFeeKey]fixedpoint.Value,
	stats *types.ProfitStats, position *types.Position,
) error {
	if len(allTrades) == 0 {
		return nil
	}

	trades := types.SortTradesAscending(allTrades)
	oldestTrade := trades[0]
	lastTrade := trades[len(trades)-1]
	// clear existing position and profit records
	if f.Environment.PositionService != nil {
		// TODO: add strategy and strategy_instance_id filter
		err := f.Environment.PositionService.Delete(service.PositionQueryOptions{
			Symbol:    position.Symbol,
			StartTime: oldestTrade.Time.Time(),
			EndTime:   lastTrade.Time.Time(),
		})
		if err != nil {
			return fmt.Errorf("failed to delete existing position records: %w", err)
		}
	}
	if f.Environment.ProfitService != nil {
		// TODO: add strategy and strategy_instance_id filter
		err := f.Environment.ProfitService.Delete(service.ProfitQueryOptions{
			Symbol:    position.Symbol,
			StartTime: oldestTrade.Time.Time(),
			EndTime:   lastTrade.Time.Time(),
		})
		if err != nil {
			return fmt.Errorf("failed to delete existing profit records: %w", err)
		}
	}
	// do fixing from trades
	for _, trade := range trades {
		trade = f.ConverterManager.ConvertTrade(trade)
		// set fee average cost
		if feePrice, ok := tokenFeePrices[tokenFeeKey{
			token:        trade.FeeCurrency,
			exchangeName: trade.Exchange,
			date:         trade.Time.Time().Format(time.DateOnly),
		}]; ok {
			position.SetFeeAverageCost(trade.FeeCurrency, feePrice)
		}
		profit, netProfit, madeProfit := position.AddTrade(trade)
		stats.AddTrade(trade)
		if madeProfit {
			p := position.NewProfit(trade, profit, netProfit)
			stats.AddProfit(p)
			f.Environment.RecordPosition(position, trade, &p)
		} else {
			f.Environment.RecordPosition(position, trade, nil)
		}
	}

	log.Infof("profitFixer fix finished: profitStats and position are updated from %d trades", len(allTrades))
	return nil
}

type ProfitFixerBundle struct {
	ProfitFixerConfig *ProfitFixerConfig `json:"profitFixer,omitempty"`

	*bbgo.Environment
}

func (f *ProfitFixerBundle) Fix(
	ctx context.Context,
	symbol string,
	position *types.Position,
	profitStats *types.ProfitStats,
	sessions ...*bbgo.ExchangeSession,
) error {
	if f.Environment == nil {
		return fmt.Errorf("environment is not set in ProfitFixerBundle")
	}
	bbgo.Notify("Fixing %s profitStats and position...", symbol)

	log.Infof("profitFixer is enabled, checking checkpoint: %+v", f.ProfitFixerConfig.TradesSince)

	if f.ProfitFixerConfig.TradesSince.Time().IsZero() {
		return fmt.Errorf("tradesSince time can not be zero")
	}

	fixer := NewProfitFixer(*f.ProfitFixerConfig, f.Environment)
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

func (f *ProfitFixer) queryTradesFromDB(ctx context.Context, symbol string, since, until time.Time) ([]types.Trade, error) {
	if f.Environment.TradeService == nil {
		return nil, fmt.Errorf("trade service is not available in the environment: %s", symbol)
	}
	var trades []types.Trade
	if symbol == "" {
		return nil, fmt.Errorf("symbol can not be empty")
	}
	for sessionName, s := range f.sessions {
		options := service.QueryTradesOptions{
			Symbol: symbol,
			Since:  &since,
			Until:  &until,
		}
		if ex, ok := s.(types.Exchange); ok {
			exchangeName := ex.Name()
			if exchangeName == "" {
				log.Warnf("skip empty exchange name for session: %s", sessionName)
				continue
			}
			options.Exchange = exchangeName
			isMargin, isFutures, isIsolated, isolatedSymbol := exchange.GetSessionAttributes(ex)
			options.IsMargin = &isMargin
			options.IsFutures = &isFutures
			if isolatedSymbol == symbol {
				options.IsIsolated = &isIsolated
			}
		} else {
			log.Warnf("session does not implement types.Exchange, skipping: %s", sessionName)
			continue
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			trades_, err := f.Environment.TradeService.Query(options)
			if err != nil {
				return nil, err
			}
			trades = append(trades, trades_...)
		}
	}
	return trades, nil
}

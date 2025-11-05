package common

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

// ProfitFixerConfig is used for fixing profitStats and position by re-playing the trade history
type ProfitFixerConfig struct {
	TradesSince       types.Time `json:"tradesSince,omitempty"`
	Patch             string     `json:"patch,omitempty"`
	UseDatabaseTrades bool       `json:"useDatabaseTrades,omitempty"`
}

func (c ProfitFixerConfig) Equal(other ProfitFixerConfig) bool {
	return c.TradesSince.Equal(other.TradesSince.Time()) && c.Patch == other.Patch && c.UseDatabaseTrades == other.UseDatabaseTrades
}

// ProfitFixer implements a trade-history-based profit fixer
type ProfitFixer struct {
	sessions map[string]types.ExchangeTradeHistoryService

	core.ConverterManager
	*bbgo.Environment
}

type tokenFeeKey struct {
	token        string
	exchangeName types.ExchangeName
	date         string
}

func NewProfitFixer(environment *bbgo.Environment) *ProfitFixer {
	return &ProfitFixer{
		sessions:    make(map[string]types.ExchangeTradeHistoryService),
		Environment: environment,
	}
}

func (f *ProfitFixer) GetConverter() *core.ConverterManager {
	return &f.ConverterManager
}

func (f *ProfitFixer) GetEnvironment() *bbgo.Environment {
	return f.Environment
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

func buildTokenFeeDatePrices(ctx context.Context, sessions map[string]types.ExchangeTradeHistoryService, trades []types.Trade, since, until time.Time) (map[tokenFeeKey]fixedpoint.Value, error) {
	// initialize tokenFeePrices map
	tokenFeePrices := make(map[tokenFeeKey]fixedpoint.Value)

	if len(trades) == 0 {
		return tokenFeePrices, nil
	}

	// token -> symbol, exchangeName
	tokens := make(map[tokenFeeKey]struct{})
	// exchangeName -> markets
	markets := make(map[types.ExchangeName]types.MarketMap)
	// exchangeName -> ExchangePublic: query required data by trade.Exchange
	exchanges := make(map[types.ExchangeName]types.Exchange)
	for sessionName, service := range sessions {
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

	// quote currency is assumed to be the same for all trades
	var quoteCurrency string
	// usdFeeExchanges is the set of exchanges that have USD fee currency
	usdFeeExchanges := map[types.ExchangeName]struct{}{}
	// populate tokens map and usdFeeExchanges
	// tokens map should only include non-USD stablecoin tokens. ex: BNB
	for _, trade := range trades {
		// skip trade if fee currency is USD* (ex: USD, USDT, USDC, ...)
		if strings.HasPrefix(trade.FeeCurrency, "USD") {
			// record exchanges that have USD fee currency
			if trade.FeeCurrency == "USD" {
				usdFeeExchanges[trade.Exchange] = struct{}{}
			}
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
			return nil, fmt.Errorf("quote currency mismatch: %s != %s", quoteCurrency, market.QuoteCurrency)
		}
		quoteCurrency = market.QuoteCurrency
		tokens[tokenFeeKey{
			token:        trade.FeeCurrency,
			exchangeName: trade.Exchange,
			date:         "", // date is a dummy here
		}] = struct{}{}
	}
	// no quote currency found if:
	// - all fees are stablecoins (USDT, USDC, ...etc), or
	// - all fees are base currency
	// - no fee charged in USD
	// no need to build token fee map in this case
	if len(tokens) == 0 && len(usdFeeExchanges) == 0 {
		return tokenFeePrices, nil
	}
	startTime := since.Truncate(24 * time.Hour).Add(-24 * time.Hour)
	endTime := until.Truncate(24 * time.Hour)
	for info := range tokens {
		ex, ok := exchanges[info.exchangeName]
		if !ok {
			log.Warnf("can not build token fee on exchange %s: %s", info.exchangeName, info.token)
			continue
		}
		markets, ok := markets[info.exchangeName]
		if !ok {
			log.Warnf("cant not find markets for exchange %s", info.exchangeName)
		}
		market, ok := markets.FindPair(info.token, quoteCurrency)
		if !ok {
			log.Warnf("can not find market for token for %s pair on %s: %s", info.token, quoteCurrency, info.exchangeName)
			continue
		}
		if klines, err := queryKLines(
			ctx, ex, market.Symbol, types.Interval1d, startTime, endTime,
		); err == nil {
			for _, kline := range klines {
				// the date in tokenFeeKey is the next day of the kline date
				// which means the token fee for the next day is calculated by the previous day's closing price
				tokenFeePrices[tokenFeeKey{
					token:        info.token,
					exchangeName: info.exchangeName,
					date:         kline.StartTime.Time().Add(24 * time.Hour).Format(time.DateOnly),
				}] = kline.Close
			}
		} else {
			return nil, err
		}
	}
	// USD prices
	// only build USD prices if there are trades with USD* fee currency
	// ex: USDT, USDC
	for exName := range usdFeeExchanges {
		ex, ok := exchanges[exName]
		if !ok {
			continue
		}
		markets, ok := markets[exName]
		if !ok {
			continue
		}
		quoteMarket, ok := markets.FindPair(quoteCurrency, "USD")
		if !ok {
			continue
		}
		if klines, err := queryKLines(
			ctx, ex, quoteMarket.Symbol, types.Interval1d, startTime, endTime,
		); err == nil {
			for _, kline := range klines {
				// we still use the last day's closing price to calculate the USD fee price.
				// This is different from the actual fee calculation while trading which use the close price of the last minute.
				// But assuming the price of USD stablecoins do not fluctuate much within a day,
				// this should be acceptable.
				tokenFeePrices[tokenFeeKey{
					token:        "USD",
					exchangeName: ex.Name(),
					date:         kline.StartTime.Time().Add(24 * time.Hour).Format(time.DateOnly),
				}] = fixedpoint.One.Div(kline.Close)
			}
		} else {
			continue
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
	log.Infof("starting profitFixer with time range %s <=> %s", since, until)
	allTrades, err := f.aggregateAllTrades(ctx, symbol, since, until)
	if err != nil {
		return err
	}
	if len(allTrades) == 0 {
		log.Warnf("[%s] no trades found between %s and %s, skip profit fixing", symbol, since.String(), until.String())
		return nil
	}
	fm, err := buildTokenFeeDatePrices(ctx, f.sessions, allTrades, since, until)
	if err != nil {
		return err
	}
	return fixFromTrades(f, allTrades, fm, stats, position)
}

type iFixer interface {
	GetEnvironment() *bbgo.Environment
	GetConverter() *core.ConverterManager
}

func fixFromTrades(
	fixer iFixer,
	allTrades []types.Trade, tokenFeePrices map[tokenFeeKey]fixedpoint.Value,
	stats *types.ProfitStats, position *types.Position,
) error {
	if len(allTrades) == 0 {
		return nil
	}
	environ := fixer.GetEnvironment()
	converter := fixer.GetConverter()

	trades := types.SortTradesAscending(allTrades)
	oldestTrade := trades[0]
	lastTrade := trades[len(trades)-1]
	// clear existing position and profit records
	if environ.PositionService != nil {
		// TODO: add strategy and strategy_instance_id filter
		err := environ.PositionService.Delete(service.PositionQueryOptions{
			Symbol:    position.Symbol,
			StartTime: oldestTrade.Time.Time(),
			EndTime:   lastTrade.Time.Time(),
		})
		if err != nil {
			return fmt.Errorf("failed to delete existing position records: %w", err)
		}
	}
	if environ.ProfitService != nil {
		// TODO: add strategy and strategy_instance_id filter
		err := environ.ProfitService.Delete(service.ProfitQueryOptions{
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
		if converter != nil {
			trade = converter.ConvertTrade(trade)
		}
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
			if position.Strategy != "" {
				p.Strategy = position.Strategy
			}
			if position.StrategyInstanceID != "" {
				p.StrategyInstanceID = position.StrategyInstanceID
			}
			stats.AddProfit(p)
			environ.RecordPosition(position, trade, &p)
		} else {
			environ.RecordPosition(position, trade, nil)
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

	fixer := NewProfitFixer(f.Environment)
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

type DatabaseProfitFixer struct {
	sessions map[string]types.ExchangeTradeHistoryService

	core.ConverterManager
	*bbgo.Environment
}

func NewDBProfitFixer(environment *bbgo.Environment) *DatabaseProfitFixer {
	return &DatabaseProfitFixer{
		Environment: environment,
		sessions:    make(map[string]types.ExchangeTradeHistoryService),
	}
}

func (f *DatabaseProfitFixer) AddExchange(sessionName string, service types.ExchangeTradeHistoryService) {
	f.sessions[sessionName] = service
}

func (f *DatabaseProfitFixer) GetConverter() *core.ConverterManager {
	return &f.ConverterManager
}

func (f *DatabaseProfitFixer) GetEnvironment() *bbgo.Environment {
	return f.Environment
}

func (f *DatabaseProfitFixer) Fix(
	ctx context.Context, symbol string, since, until time.Time, stats *types.ProfitStats, position *types.Position,
) error {
	log.Infof("starting profit fixer with time range %s <=> %s (from DB)", since, until)
	allTrades, err := f.queryAllTrades(ctx, symbol, since, until)
	if err != nil {
		return err
	}
	if len(allTrades) == 0 {
		log.Warnf("[%s] no trades found between %s and %s, skip profit fixing", symbol, since.String(), until.String())
		return nil
	}
	fm, err := buildTokenFeeDatePrices(ctx, f.sessions, allTrades, since, until)
	if err != nil {
		return err
	}
	return fixFromTrades(f, allTrades, fm, stats, position)
}

func (f *DatabaseProfitFixer) queryAllTrades(ctx context.Context, symbol string, since, until time.Time) ([]types.Trade, error) {
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

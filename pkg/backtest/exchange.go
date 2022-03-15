/*
The backtest process

The backtest engine loads the klines from the database into a kline-channel,
there are multiple matching engine that matches the order sent from the strategy.

for each kline, the backtest engine:

1) load the kline, run matching logics to send out order update and trades to the user data stream.
2) once the matching process for the kline is done, the kline will be pushed to the market data stream.
3) go to 1 and load the next kline.

There are 2 ways that a strategy could work with backtest engine:

1. the strategy receives kline from the market data stream, and then it submits the order by the given market data to the backtest engine.
   backtest engine receives the order and then pushes the trade and order updates to the user data stream.

   the strategy receives the trade and update its position.

2. the strategy places the orders when it starts. (like grid) the strategy then receives the order updates and then submit a new order
   by its order update message.

We need to ensure that:

1. if the strategy submits the order from the market data stream, since it's a separate goroutine, the strategy should block the backtest engine
   to process the trades before the next kline is published.
*/
package backtest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/cache"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

var ErrUnimplemented = errors.New("unimplemented method")

type Exchange struct {
	sourceName         types.ExchangeName
	publicExchange     types.Exchange
	srv                *service.BacktestService
	startTime, endTime time.Time

	account *types.Account
	config  *bbgo.Backtest

	userDataStream, marketDataStream *Stream

	trades      map[string][]types.Trade
	tradesMutex sync.Mutex

	closedOrders      map[string][]types.Order
	closedOrdersMutex sync.Mutex

	matchingBooks      map[string]*SimplePriceMatching
	matchingBooksMutex sync.Mutex

	markets types.MarketMap
}

func NewExchange(sourceName types.ExchangeName, sourceExchange types.Exchange, srv *service.BacktestService, config *bbgo.Backtest) (*Exchange, error) {
	ex := sourceExchange

	markets, err := cache.LoadExchangeMarketsWithCache(context.Background(), ex)
	if err != nil {
		return nil, err
	}

	var startTime, endTime time.Time
	startTime = config.StartTime.Time()
	if config.EndTime != nil {
		endTime = config.EndTime.Time()
	} else {
		endTime = time.Now()
	}

	configAccount := config.Account[sourceName.String()]

	account := &types.Account{
		MakerFeeRate: configAccount.MakerFeeRate,
		TakerFeeRate: configAccount.TakerFeeRate,
		AccountType:  types.AccountTypeSpot,
	}

	balances := configAccount.Balances.BalanceMap()
	account.UpdateBalances(balances)

	e := &Exchange{
		sourceName:     sourceName,
		publicExchange: ex,
		markets:        markets,
		srv:            srv,
		config:         config,
		account:        account,
		startTime:      startTime,
		endTime:        endTime,
		closedOrders:   make(map[string][]types.Order),
		trades:         make(map[string][]types.Trade),
	}

	e.resetMatchingBooks()
	return e, nil
}

func (e *Exchange) addTrade(trade types.Trade) {
	e.tradesMutex.Lock()
	e.trades[trade.Symbol] = append(e.trades[trade.Symbol], trade)
	e.tradesMutex.Unlock()
}

func (e *Exchange) addClosedOrder(order types.Order) {
	e.closedOrdersMutex.Lock()
	e.closedOrders[order.Symbol] = append(e.closedOrders[order.Symbol], order)
	e.closedOrdersMutex.Unlock()
}

func (e *Exchange) resetMatchingBooks() {
	e.matchingBooksMutex.Lock()
	e.matchingBooks = make(map[string]*SimplePriceMatching)
	for symbol, market := range e.markets {
		e._addMatchingBook(symbol, market)
	}
	e.matchingBooksMutex.Unlock()
}

func (e *Exchange) addMatchingBook(symbol string, market types.Market) {
	e.matchingBooksMutex.Lock()
	e._addMatchingBook(symbol, market)
	e.matchingBooksMutex.Unlock()
}

func (e *Exchange) _addMatchingBook(symbol string, market types.Market) {
	e.matchingBooks[symbol] = &SimplePriceMatching{
		CurrentTime: e.startTime,
		Account:     e.account,
		Market:      market,
	}
}

func (e *Exchange) NewStream() types.Stream {
	return &Stream{exchange: e}
}

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	if e.userDataStream == nil {
		return createdOrders, fmt.Errorf("SubmitOrders should be called after userDataStream been initialized")
	}
	for _, order := range orders {
		symbol := order.Symbol
		matching, ok := e.matchingBook(symbol)
		if !ok {
			return nil, fmt.Errorf("matching engine is not initialized for symbol %s", symbol)
		}

		createdOrder, _, err := matching.PlaceOrder(order)
		if err != nil {
			return nil, err
		}

		if createdOrder != nil {
			createdOrders = append(createdOrders, *createdOrder)

			// market order can be closed immediately.
			switch createdOrder.Status {
			case types.OrderStatusFilled, types.OrderStatusCanceled, types.OrderStatusRejected:
				e.addClosedOrder(*createdOrder)
			}

			e.userDataStream.EmitOrderUpdate(*createdOrder)
		}
	}

	return createdOrders, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	matching, ok := e.matchingBook(symbol)
	if !ok {
		return nil, fmt.Errorf("matching engine is not initialized for symbol %s", symbol)
	}

	return append(matching.bidOrders, matching.askOrders...), nil
}

func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	orders, ok := e.closedOrders[symbol]
	if !ok {
		return orders, fmt.Errorf("matching engine is not initialized for symbol %s", symbol)
	}

	return orders, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	if e.userDataStream == nil {
		return fmt.Errorf("CancelOrders should be called after userDataStream been initialized")
	}
	for _, order := range orders {
		matching, ok := e.matchingBook(order.Symbol)
		if !ok {
			return fmt.Errorf("matching engine is not initialized for symbol %s", order.Symbol)
		}
		canceledOrder, err := matching.CancelOrder(order)
		if err != nil {
			return err
		}

		e.userDataStream.EmitOrderUpdate(canceledOrder)
	}

	return nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	return e.account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	return e.account.Balances(), nil
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	if options.EndTime != nil {
		return e.srv.QueryKLinesBackward(e.sourceName, symbol, interval, *options.EndTime, 1000)
	}

	if options.StartTime != nil {
		return e.srv.QueryKLinesForward(e.sourceName, symbol, interval, *options.StartTime, 1000)
	}

	return nil, errors.New("endTime or startTime can not be nil")
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	// we don't need query trades for backtest
	return nil, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	matching, ok := e.matchingBook(symbol)
	if !ok {
		return nil, fmt.Errorf("matching engine is not initialized for symbol %s", symbol)
	}

	kline := matching.LastKLine
	return &types.Ticker{
		Time:   kline.EndTime.Time(),
		Volume: kline.Volume,
		Last:   kline.Close,
		Open:   kline.Open,
		High:   kline.High,
		Low:    kline.Low,
		Buy:    kline.Close,
		Sell:   kline.Close,
	}, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	// Not using Tickers in back test (yet)
	return nil, ErrUnimplemented
}

func (e *Exchange) Name() types.ExchangeName {
	return e.publicExchange.Name()
}

func (e *Exchange) PlatformFeeCurrency() string {
	return e.publicExchange.PlatformFeeCurrency()
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	return e.markets, nil
}

func (e Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	return nil, nil
}

func (e Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
	return nil, nil
}

func (e *Exchange) matchingBook(symbol string) (*SimplePriceMatching, bool) {
	e.matchingBooksMutex.Lock()
	m, ok := e.matchingBooks[symbol]
	e.matchingBooksMutex.Unlock()
	return m, ok
}

func (e *Exchange) InitMarketData() {
	e.userDataStream.OnTradeUpdate(func(trade types.Trade) {
		e.addTrade(trade)
	})

	e.matchingBooksMutex.Lock()
	for _, matching := range e.matchingBooks {
		matching.OnTradeUpdate(e.userDataStream.EmitTradeUpdate)
		matching.OnOrderUpdate(e.userDataStream.EmitOrderUpdate)
		matching.OnBalanceUpdate(e.userDataStream.EmitBalanceUpdate)
	}
	e.matchingBooksMutex.Unlock()

}

func (e *Exchange) GetMarketData() (chan types.KLine, error) {
	log.Infof("collecting backtest configurations...")

	loadedSymbols := map[string]struct{}{}
	loadedIntervals := map[types.Interval]struct{}{
		// 1m interval is required for the backtest matching engine
		types.Interval1m: {},
		types.Interval1d: {},
	}
	for _, sub := range e.marketDataStream.Subscriptions {
		loadedSymbols[sub.Symbol] = struct{}{}

		switch sub.Channel {
		case types.KLineChannel:
			loadedIntervals[types.Interval(sub.Options.Interval)] = struct{}{}

		default:
			return nil, fmt.Errorf("stream channel %s is not supported in backtest", sub.Channel)
		}
	}

	var symbols []string
	for symbol := range loadedSymbols {
		symbols = append(symbols, symbol)
	}

	var intervals []types.Interval
	for interval := range loadedIntervals {
		intervals = append(intervals, interval)
	}

	log.Infof("using symbols: %v and intervals: %v for back-testing", symbols, intervals)
	log.Infof("querying klines from database...")
	klineC, errC := e.srv.QueryKLinesCh(e.startTime, e.endTime, e, symbols, intervals)
	go func() {
		if err := <-errC; err != nil {
			log.WithError(err).Error("backtest data feed error")
		}
	}()
	return klineC, nil
}

func (e *Exchange) ConsumeKLine(k types.KLine) {
	if k.Interval == types.Interval1m {
		matching, ok := e.matchingBook(k.Symbol)
		if !ok {
			log.Errorf("matching book of %s is not initialized", k.Symbol)
			return
		}

		// here we generate trades and order updates
		matching.processKLine(k)
	}

	e.marketDataStream.EmitKLineClosed(k)
}

func (e *Exchange) CloseMarketData() error {
	if err := e.marketDataStream.Close(); err != nil {
		log.WithError(err).Error("stream close error")
		return err
	}
	return nil
}

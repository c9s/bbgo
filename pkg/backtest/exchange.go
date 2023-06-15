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
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/cache"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("cmd", "backtest")

var ErrUnimplemented = errors.New("unimplemented method")
var ErrNegativeQuantity = errors.New("order quantity can not be negative")
var ErrZeroQuantity = errors.New("order quantity can not be zero")

type Exchange struct {
	sourceName     types.ExchangeName
	publicExchange types.Exchange
	srv            *service.BacktestService
	currentTime    time.Time

	account *types.Account
	config  *bbgo.Backtest

	MarketDataStream types.StandardStreamEmitter

	trades      map[string][]types.Trade
	tradesMutex sync.Mutex

	closedOrders      map[string][]types.Order
	closedOrdersMutex sync.Mutex

	matchingBooks      map[string]*SimplePriceMatching
	matchingBooksMutex sync.Mutex

	markets types.MarketMap

	Src *ExchangeDataSource
}

func NewExchange(sourceName types.ExchangeName, sourceExchange types.Exchange, srv *service.BacktestService, config *bbgo.Backtest) (*Exchange, error) {
	ex := sourceExchange

	markets, err := cache.LoadExchangeMarketsWithCache(context.Background(), ex)
	if err != nil {
		return nil, err
	}

	startTime := config.StartTime.Time()
	configAccount := config.GetAccount(sourceName.String())

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
		currentTime:    startTime,
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
	matching := &SimplePriceMatching{
		currentTime:     e.currentTime,
		account:         e.account,
		Market:          market,
		closedOrders:    make(map[uint64]types.Order),
		feeModeFunction: getFeeModeFunction(e.config.FeeMode),
	}

	e.matchingBooks[symbol] = matching
}

func (e *Exchange) NewStream() types.Stream {
	return &types.BacktestStream{
		StandardStreamEmitter: &types.StandardStream{},
	}
}

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	book := e.matchingBooks[q.Symbol]
	oid, err := strconv.ParseUint(q.OrderID, 10, 64)
	if err != nil {
		return nil, err
	}

	order, ok := book.getOrder(oid)
	if ok {
		return &order, nil
	}
	return nil, nil
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	symbol := order.Symbol
	matching, ok := e.matchingBook(symbol)
	if !ok {
		return nil, fmt.Errorf("matching engine is not initialized for symbol %s", symbol)
	}

	if order.Quantity.Sign() < 0 {
		return nil, ErrNegativeQuantity
	}

	if order.Quantity.IsZero() {
		return nil, ErrZeroQuantity
	}

	createdOrder, _, err = matching.PlaceOrder(order)
	if createdOrder != nil {
		// market order can be closed immediately.
		switch createdOrder.Status {
		case types.OrderStatusFilled, types.OrderStatusCanceled, types.OrderStatusRejected:
			e.addClosedOrder(*createdOrder)
		}
	}

	return createdOrder, err
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
	for _, order := range orders {
		matching, ok := e.matchingBook(order.Symbol)
		if !ok {
			return fmt.Errorf("matching engine is not initialized for symbol %s", order.Symbol)
		}
		_, err := matching.CancelOrder(order)
		if err != nil {
			return err
		}
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

	kline := matching.lastKLine
	return &types.Ticker{
		Time:   kline.EndTime.Time(),
		Volume: kline.Volume,
		Last:   kline.Close,
		Open:   kline.Open,
		High:   kline.High,
		Low:    kline.Low,
		Buy:    kline.Close.Sub(matching.Market.TickSize),
		Sell:   kline.Close.Add(matching.Market.TickSize),
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

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	return nil, nil
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
	return nil, nil
}

func (e *Exchange) matchingBook(symbol string) (*SimplePriceMatching, bool) {
	e.matchingBooksMutex.Lock()
	m, ok := e.matchingBooks[symbol]
	e.matchingBooksMutex.Unlock()
	return m, ok
}

func (e *Exchange) BindUserData(userDataStream types.StandardStreamEmitter) {
	userDataStream.OnTradeUpdate(func(trade types.Trade) {
		e.addTrade(trade)
	})

	e.matchingBooksMutex.Lock()
	for _, matching := range e.matchingBooks {
		matching.OnTradeUpdate(userDataStream.EmitTradeUpdate)
		matching.OnOrderUpdate(userDataStream.EmitOrderUpdate)
		matching.OnBalanceUpdate(userDataStream.EmitBalanceUpdate)
	}
	e.matchingBooksMutex.Unlock()
}

func (e *Exchange) SubscribeMarketData(startTime, endTime time.Time, requiredInterval types.Interval, extraIntervals ...types.Interval) (chan types.KLine, error) {
	log.Infof("collecting backtest configurations...")

	loadedSymbols := map[string]struct{}{}
	loadedIntervals := map[types.Interval]struct{}{
		// 1m interval is required for the backtest matching engine
		requiredInterval: {},
	}

	for _, it := range extraIntervals {
		loadedIntervals[it] = struct{}{}
	}

	// collect subscriptions
	for _, sub := range e.MarketDataStream.GetSubscriptions() {
		loadedSymbols[sub.Symbol] = struct{}{}

		switch sub.Channel {
		case types.KLineChannel:
			loadedIntervals[sub.Options.Interval] = struct{}{}

		default:
			// Since Environment is not yet been injected at this point, no hard error
			log.Errorf("stream channel %s is not supported in backtest", sub.Channel)
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

	log.Infof("querying klines from database with exchange: %v symbols: %v and intervals: %v for back-testing", e.Name(), symbols, intervals)
	klineC, errC := e.srv.QueryKLinesCh(startTime, endTime, e, symbols, intervals)
	go func() {
		if err := <-errC; err != nil {
			log.WithError(err).Error("backtest data feed error")
		}
	}()
	return klineC, nil
}

func (e *Exchange) ConsumeKLine(k types.KLine, requiredInterval types.Interval) {
	matching, ok := e.matchingBook(k.Symbol)
	if !ok {
		log.Errorf("matching book of %s is not initialized", k.Symbol)
		return
	}
	if matching.klineCache == nil {
		matching.klineCache = make(map[types.Interval]types.KLine)
	}

	requiredKline, ok := matching.klineCache[k.Interval]
	if ok { // pop out all the old
		if requiredKline.Interval != requiredInterval {
			panic(fmt.Sprintf("expect required kline interval %s, got interval %s", requiredInterval.String(), requiredKline.Interval.String()))
		}
		e.currentTime = requiredKline.EndTime.Time()
		// here we generate trades and order updates
		matching.processKLine(requiredKline)
		matching.nextKLine = &k
		for _, kline := range matching.klineCache {
			e.MarketDataStream.EmitKLineClosed(kline)
			for _, h := range e.Src.Callbacks {
				h(kline, e.Src)
			}
		}
		// reset the paramcache
		matching.klineCache = make(map[types.Interval]types.KLine)
	}
	matching.klineCache[k.Interval] = k
}

func (e *Exchange) CloseMarketData() error {
	if err := e.MarketDataStream.Close(); err != nil {
		log.WithError(err).Error("stream close error")
		return err
	}
	return nil
}

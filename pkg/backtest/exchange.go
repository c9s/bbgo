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

	"github.com/c9s/bbgo/pkg/exchange/ftx"
	"github.com/c9s/bbgo/pkg/exchange/okex"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/max"
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

	userDataStream *Stream

	trades      map[string][]types.Trade
	tradesMutex sync.Mutex

	closedOrders      map[string][]types.Order
	closedOrdersMutex sync.Mutex

	matchingBooks      map[string]*SimplePriceMatching
	matchingBooksMutex sync.Mutex

	markets types.MarketMap
	doneC   chan struct{}
}

func NewExchange(sourceName types.ExchangeName, srv *service.BacktestService, config *bbgo.Backtest) (*Exchange, error) {
	ex, err := newPublicExchange(sourceName)
	if err != nil {
		return nil, err
	}

	if config == nil {
		return nil, errors.New("backtest config can not be nil")
	}

	markets, err := bbgo.LoadExchangeMarketsWithCache(context.Background(), ex)
	if err != nil {
		return nil, err
	}

	startTime, err := config.ParseStartTime()
	if err != nil {
		return nil, err
	}

	endTime, err := config.ParseEndTime()
	if err != nil {
		return nil, err
	}

	account := &types.Account{
		MakerFeeRate: config.Account.MakerFeeRate,
		TakerFeeRate: config.Account.TakerFeeRate,
		AccountType:  "SPOT", // currently not used
	}

	balances := config.Account.Balances.BalanceMap()
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
		doneC:          make(chan struct{}),
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

func (e *Exchange) Done() chan struct{} {
	return e.doneC
}

func (e *Exchange) NewStream() types.Stream {
	return &Stream{exchange: e}
}

func (e Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	for _, order := range orders {
		symbol := order.Symbol
		matching, ok := e.matchingBook(symbol)
		if !ok {
			return nil, fmt.Errorf("matching engine is not initialized for symbol %s", symbol)
		}

		createdOrder, trade, err := matching.PlaceOrder(order)
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

		if trade != nil {
			e.userDataStream.EmitTradeUpdate(*trade)
		}
	}

	return createdOrders, nil
}

func (e Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	matching, ok := e.matchingBook(symbol)
	if !ok {
		return nil, fmt.Errorf("matching engine is not initialized for symbol %s", symbol)
	}

	return append(matching.bidOrders, matching.askOrders...), nil
}

func (e Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	orders, ok := e.closedOrders[symbol]
	if !ok {
		return orders, fmt.Errorf("matching engine is not initialized for symbol %s", symbol)
	}

	return orders, nil
}

func (e Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
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

func (e Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	return e.account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	return e.account.Balances(), nil
}

func (e Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	if options.EndTime != nil {
		return e.srv.QueryKLinesBackward(e.sourceName, symbol, interval, *options.EndTime, 1000)
	}

	if options.StartTime != nil {
		return e.srv.QueryKLinesForward(e.sourceName, symbol, interval, *options.StartTime, 1000)
	}

	return nil, errors.New("endTime or startTime can not be nil")
}

func (e Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	// we don't need query trades for backtest
	return nil, nil
}

func (e Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	matching, ok := e.matchingBook(symbol)
	if !ok {
		return nil, fmt.Errorf("matching engine is not initialized for symbol %s", symbol)
	}

	kline := matching.LastKLine
	return &types.Ticker{
		Time:   kline.EndTime,
		Volume: kline.Volume,
		Last:   kline.Close,
		Open:   kline.Open,
		High:   kline.High,
		Low:    kline.Low,
		Buy:    kline.Close,
		Sell:   kline.Close,
	}, nil
}

func (e Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	// Not using Tickers in back test (yet)
	return nil, ErrUnimplemented
}

func (e Exchange) Name() types.ExchangeName {
	return e.publicExchange.Name()
}

func (e Exchange) PlatformFeeCurrency() string {
	return e.publicExchange.PlatformFeeCurrency()
}

func (e Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	return bbgo.LoadExchangeMarketsWithCache(ctx, e.publicExchange)
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

func newPublicExchange(sourceExchange types.ExchangeName) (types.Exchange, error) {
	switch sourceExchange {
	case types.ExchangeBinance:
		return binance.New("", ""), nil
	case types.ExchangeMax:
		return max.New("", ""), nil
	case types.ExchangeFTX:
		return ftx.NewExchange("", "", ""), nil
	case types.ExchangeOKEx:
		return okex.New("", "", ""), nil
	}

	return nil, fmt.Errorf("public data from exchange %s is not supported", sourceExchange)
}

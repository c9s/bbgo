package backtest

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

type Exchange struct {
	sourceName         types.ExchangeName
	publicExchange     types.Exchange
	srv                *service.BacktestService
	startTime, endTime time.Time

	account *types.Account
	config  *bbgo.Backtest

	stream *Stream

	trades        map[string][]types.Trade
	closedOrders  map[string][]types.Order
	matchingBooks map[string]*SimplePriceMatching
	markets       types.MarketMap
	doneC         chan struct{}
}

func NewExchange(sourceName types.ExchangeName, srv *service.BacktestService, config *bbgo.Backtest) *Exchange {
	ex, err := newPublicExchange(sourceName)
	if err != nil {
		panic(err)
	}

	if config == nil {
		panic(errors.New("backtest config can not be nil"))
	}

	markets, err := bbgo.LoadExchangeMarketsWithCache(context.Background(), ex)
	if err != nil {
		panic(err)
	}

	startTime, err := config.ParseStartTime()
	if err != nil {
		panic(err)
	}

	endTime, err := config.ParseEndTime()
	if err != nil {
		panic(err)
	}

	account := &types.Account{
		MakerCommission: config.Account.MakerCommission,
		TakerCommission: config.Account.TakerCommission,
		AccountType:     "SPOT", // currently not used
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
		matchingBooks:  make(map[string]*SimplePriceMatching),
		closedOrders:   make(map[string][]types.Order),
		trades:         make(map[string][]types.Trade),
		doneC:          make(chan struct{}),
	}

	return e
}

func (e *Exchange) Done() chan struct{} {
	return e.doneC
}

func (e *Exchange) NewStream() types.Stream {
	if e.stream != nil {
		panic("backtest stream can not be allocated twice")
	}

	e.stream = &Stream{exchange: e}

	e.stream.OnTradeUpdate(func(trade types.Trade) {
		e.trades[trade.Symbol] = append(e.trades[trade.Symbol], trade)
	})

	for symbol, market := range e.markets {
		matching := &SimplePriceMatching{
			CurrentTime:     e.startTime,
			Account:         e.account,
			Market:          market,
			MakerCommission: e.config.Account.MakerCommission,
			TakerCommission: e.config.Account.TakerCommission,
		}
		matching.OnTradeUpdate(e.stream.EmitTradeUpdate)
		matching.OnOrderUpdate(e.stream.EmitOrderUpdate)
		matching.OnBalanceUpdate(e.stream.EmitBalanceUpdate)
		e.matchingBooks[symbol] = matching
	}

	return e.stream
}

func (e Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	for _, order := range orders {
		symbol := order.Symbol
		matching, ok := e.matchingBooks[symbol]
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
				e.closedOrders[symbol] = append(e.closedOrders[symbol], *createdOrder)
			}

			e.stream.EmitOrderUpdate(*createdOrder)
		}

		if trade != nil {
			e.stream.EmitTradeUpdate(*trade)
		}
	}

	return createdOrders, nil
}

func (e Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	matching, ok := e.matchingBooks[symbol]
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
		matching, ok := e.matchingBooks[order.Symbol]
		if !ok {
			return fmt.Errorf("matching engine is not initialized for symbol %s", order.Symbol)
		}
		canceledOrder, err := matching.CancelOrder(order)
		if err != nil {
			return err
		}

		e.stream.EmitOrderUpdate(canceledOrder)
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
		return e.srv.QueryKLinesBackward(e.sourceName, symbol, interval, *options.EndTime)
	}
	if options.StartTime != nil {
		return e.srv.QueryKLinesForward(e.sourceName, symbol, interval, *options.StartTime)
	}

	return nil, errors.New("endTime or startTime can not be nil")
}

func (e Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	// we don't need query trades for backtest
	return nil, nil
}

func (e Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	// Not using Tickers in back test (yet)
	return nil, nil
}

func (e Exchange) Name() types.ExchangeName {
	return e.publicExchange.Name()
}

func (e Exchange) PlatformFeeCurrency() string {
	return e.publicExchange.PlatformFeeCurrency()
}

func (e Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	return e.publicExchange.QueryMarkets(ctx)
}

func (e Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	return nil, nil
}

func (e Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
	return nil, nil
}

func newPublicExchange(sourceExchange types.ExchangeName) (types.Exchange, error) {
	switch sourceExchange {
	case types.ExchangeBinance:
		return binance.New("", ""), nil
	case types.ExchangeMax:
		return max.New("", ""), nil
	}

	return nil, fmt.Errorf("exchange %s is not supported", sourceExchange)
}

package bbgo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

type StandardIndicatorSet struct {
	Symbol string
	// Standard indicators
	// interval -> window
	sma  map[types.IntervalWindow]*indicator.SMA
	ewma map[types.IntervalWindow]*indicator.EWMA
	boll map[types.IntervalWindow]*indicator.BOLL

	store *MarketDataStore
}

func NewStandardIndicatorSet(symbol string, store *MarketDataStore) *StandardIndicatorSet {
	set := &StandardIndicatorSet{
		Symbol: symbol,
		sma:    make(map[types.IntervalWindow]*indicator.SMA),
		ewma:   make(map[types.IntervalWindow]*indicator.EWMA),
		boll:   make(map[types.IntervalWindow]*indicator.BOLL),
		store:  store,
	}

	// let us pre-defined commonly used intervals
	for interval := range types.SupportedIntervals {
		for _, window := range []int{7, 25, 99} {
			iw := types.IntervalWindow{Interval: interval, Window: window}
			set.sma[iw] = &indicator.SMA{IntervalWindow: iw}
			set.sma[iw].Bind(store)

			set.ewma[iw] = &indicator.EWMA{IntervalWindow: iw}
			set.ewma[iw].Bind(store)
		}

		// setup boll indicator, we may refactor boll indicator by subscribing SMA indicator,
		// however, since general used BOLLINGER band use window 21, which is not in the existing SMA indicator sets.
		// Pull out the bandwidth configuration as the boll Key
		iw := types.IntervalWindow{Interval: interval, Window: 21}
		set.boll[iw] = &indicator.BOLL{IntervalWindow: iw, K: 2.0}
		set.boll[iw].Bind(store)
	}

	return set
}

// BOLL returns the bollinger band indicator of the given interval and the window,
// Please note that the K for std dev is fixed and defaults to 2.0
func (set *StandardIndicatorSet) BOLL(iw types.IntervalWindow, bandWidth float64) *indicator.BOLL {
	inc, ok := set.boll[iw]
	if !ok {
		inc := &indicator.BOLL{IntervalWindow: iw, K: bandWidth}
		inc.Bind(set.store)
		set.boll[iw] = inc
	}

	return inc
}

// SMA returns the simple moving average indicator of the given interval and the window size.
func (set *StandardIndicatorSet) SMA(iw types.IntervalWindow) *indicator.SMA {
	inc, ok := set.sma[iw]
	if !ok {
		inc := &indicator.SMA{IntervalWindow: iw}
		inc.Bind(set.store)
		set.sma[iw] = inc
	}

	return inc
}

// GetEWMA returns the exponential weighed moving average indicator of the given interval and the window size.
func (set *StandardIndicatorSet) EWMA(iw types.IntervalWindow) *indicator.EWMA {
	inc, ok := set.ewma[iw]
	if !ok {
		inc := &indicator.EWMA{IntervalWindow: iw}
		inc.Bind(set.store)
		set.ewma[iw] = inc
	}

	return inc
}

// ExchangeSession presents the exchange connection Session
// It also maintains and collects the data returned from the stream.
type ExchangeSession struct {
	// exchange Session based notification system
	// we make it as a value field so that we can configure it separately
	Notifiability `json:"-"`

	// Exchange Session name
	Name string `json:"name"`

	// The exchange account states
	Account *types.Account `json:"account"`

	IsMargin bool `json:"isMargin"`

	IsIsolatedMargin bool `json:"isIsolatedMargin,omitempty"`

	IsolatedMarginSymbol string `json:"isolatedMarginSymbol,omitempty"`

	IsInitialized bool `json:"isInitialized"`

	// Stream is the connection stream of the exchange
	Stream types.Stream `json:"-"`

	Subscriptions map[types.Subscription]types.Subscription `json:"-"`

	Exchange types.Exchange `json:"-"`

	// markets defines market configuration of a symbol
	markets map[string]types.Market

	// startPrices is used for backtest
	startPrices map[string]float64

	lastPrices         map[string]float64
	lastPriceUpdatedAt time.Time

	// Trades collects the executed trades from the exchange
	// map: symbol -> []trade
	Trades map[string]*types.TradeSlice `json:"-"`

	// marketDataStores contains the market data store of each market
	marketDataStores map[string]*MarketDataStore

	positions map[string]*Position

	// standard indicators of each market
	standardIndicatorSets map[string]*StandardIndicatorSet

	orderStores map[string]*OrderStore

	usedSymbols        map[string]struct{}
	initializedSymbols map[string]struct{}
}

func NewExchangeSession(name string, exchange types.Exchange) *ExchangeSession {
	return &ExchangeSession{
		Notifiability: Notifiability{
			SymbolChannelRouter:  NewPatternChannelRouter(nil),
			SessionChannelRouter: NewPatternChannelRouter(nil),
			ObjectChannelRouter:  NewObjectChannelRouter(),
		},

		Name:          name,
		Exchange:      exchange,
		Stream:        exchange.NewStream(),
		Subscriptions: make(map[types.Subscription]types.Subscription),
		Account:       &types.Account{},
		Trades:        make(map[string]*types.TradeSlice),

		markets:               make(map[string]types.Market),
		startPrices:           make(map[string]float64),
		lastPrices:            make(map[string]float64),
		positions:             make(map[string]*Position),
		marketDataStores:      make(map[string]*MarketDataStore),
		standardIndicatorSets: make(map[string]*StandardIndicatorSet),
		orderStores:           make(map[string]*OrderStore),

		usedSymbols:        make(map[string]struct{}),
		initializedSymbols: make(map[string]struct{}),
	}
}

func (session *ExchangeSession) Init(ctx context.Context, environ *Environment) error {
	if session.IsInitialized {
		return errors.New("session is already initialized")
	}

	var log = log.WithField("session", session.Name)

	var markets, err = LoadExchangeMarketsWithCache(ctx, session.Exchange)
	if err != nil {
		return err
	}

	if len(markets) == 0 {
		return fmt.Errorf("market config should not be empty")
	}

	session.markets = markets

	// initialize balance data
	log.Infof("querying balances from session %s...", session.Name)
	balances, err := session.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	log.Infof("%s account", session.Name)
	balances.Print()

	session.Account.UpdateBalances(balances)
	session.Account.BindStream(session.Stream)
	session.Stream.OnBalanceUpdate(func(balances types.BalanceMap) {
		log.Infof("balance update: %+v", balances)
	})

	// insert trade into db right before everything
	if environ.TradeService != nil {
		session.Stream.OnTradeUpdate(func(trade types.Trade) {
			if err := environ.TradeService.Insert(trade); err != nil {
				log.WithError(err).Errorf("trade insert error: %+v", trade)
			}
		})
	}

	session.Stream.OnKLineClosed(func(kline types.KLine) {
		log.Infof("kline closed: %+v", kline)
	})

	// update last prices
	session.Stream.OnKLineClosed(func(kline types.KLine) {
		if _, ok := session.startPrices[kline.Symbol]; !ok {
			session.startPrices[kline.Symbol] = kline.Open
		}

		session.lastPrices[kline.Symbol] = kline.Close
	})

	session.IsInitialized = true
	return nil
}

// InitSymbols uses usedSymbols to initialize the related data structure
func (session *ExchangeSession) InitSymbols(ctx context.Context, environ *Environment) error {
	for symbol := range session.usedSymbols {
		// skip initialized symbols
		if _, ok := session.initializedSymbols[symbol]; ok {
			continue
		}

		if err := session.InitSymbol(ctx, environ, symbol); err != nil {
			return err
		}
	}

	return nil
}

// InitSymbol loads trades for the symbol, bind stream callbacks, init positions, market data store.
// please note, InitSymbol can not be called for the same symbol for twice
func (session *ExchangeSession) InitSymbol(ctx context.Context, environ *Environment, symbol string) error {
	if _, ok := session.initializedSymbols[symbol]; ok {
		return fmt.Errorf("symbol %s is already initialized", symbol)
	}

	market, ok := session.markets[symbol]
	if !ok {
		return fmt.Errorf("market %s is not defined", symbol)
	}

	var err error
	var trades []types.Trade
	if environ.TradeSync != nil {
		log.Infof("syncing trades from %s for symbol %s...", session.Exchange.Name(), symbol)
		if err := environ.TradeSync.SyncTrades(ctx, session.Exchange, symbol, environ.tradeScanTime); err != nil {
			return err
		}

		tradingFeeCurrency := session.Exchange.PlatformFeeCurrency()
		if strings.HasPrefix(symbol, tradingFeeCurrency) {
			trades, err = environ.TradeService.QueryForTradingFeeCurrency(session.Exchange.Name(), symbol, tradingFeeCurrency)
		} else {
			trades, err = environ.TradeService.Query(service.QueryTradesOptions{
				Exchange: session.Exchange.Name(),
				Symbol:   symbol,
			})
		}

		if err != nil {
			return err
		}

		log.Infof("symbol %s: %d trades loaded", symbol, len(trades))
	}

	session.Trades[symbol] = &types.TradeSlice{Trades: trades}
	session.Stream.OnTradeUpdate(func(trade types.Trade) {
		session.Trades[symbol].Append(trade)
	})

	position := &Position{
		Symbol:        symbol,
		BaseCurrency:  market.BaseCurrency,
		QuoteCurrency: market.QuoteCurrency,
	}
	position.AddTrades(trades)
	position.BindStream(session.Stream)
	session.positions[symbol] = position

	orderStore := NewOrderStore(symbol)
	orderStore.AddOrderUpdate = true

	orderStore.BindStream(session.Stream)
	session.orderStores[symbol] = orderStore

	marketDataStore := NewMarketDataStore(symbol)
	marketDataStore.BindStream(session.Stream)
	session.marketDataStores[symbol] = marketDataStore

	standardIndicatorSet := NewStandardIndicatorSet(symbol, marketDataStore)
	session.standardIndicatorSets[symbol] = standardIndicatorSet

	// used kline intervals by the given symbol
	var usedKLineIntervals = map[types.Interval]struct{}{}

	// always subscribe the 1m kline so we can make sure the connection persists.
	usedKLineIntervals[types.Interval1m] = struct{}{}

	for _, sub := range session.Subscriptions {
		if sub.Symbol == symbol && sub.Channel == types.KLineChannel {
			usedKLineIntervals[types.Interval(sub.Options.Interval)] = struct{}{}
		}
	}

	var lastPriceTime time.Time
	for interval := range usedKLineIntervals {
		// avoid querying the last unclosed kline
		endTime := environ.startTime.Add(- interval.Duration())
		kLines, err := session.Exchange.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
			EndTime: &endTime,
			Limit:   1000, // indicators need at least 100
		})
		if err != nil {
			return err
		}

		if len(kLines) == 0 {
			log.Warnf("no kline data for interval %s (end time <= %s)", interval, environ.startTime)
			continue
		}

		// update last prices by the given kline
		lastKLine := kLines[len(kLines)-1]
		if lastPriceTime == emptyTime {
			session.lastPrices[symbol] = lastKLine.Close
			lastPriceTime = lastKLine.EndTime
		} else if lastKLine.EndTime.After(lastPriceTime) {
			session.lastPrices[symbol] = lastKLine.Close
			lastPriceTime = lastKLine.EndTime
		}

		for _, k := range kLines {
			// let market data store trigger the update, so that the indicator could be updated too.
			marketDataStore.AddKLine(k)
		}
	}

	log.Infof("last price: %f", session.lastPrices[symbol])

	session.initializedSymbols[symbol] = struct{}{}
	return nil
}

func (session *ExchangeSession) StandardIndicatorSet(symbol string) (*StandardIndicatorSet, bool) {
	set, ok := session.standardIndicatorSets[symbol]
	return set, ok
}

func (session *ExchangeSession) Position(symbol string) (pos *Position, ok bool) {
	pos, ok = session.positions[symbol]
	return pos, ok
}

// MarketDataStore returns the market data store of a symbol
func (session *ExchangeSession) MarketDataStore(symbol string) (s *MarketDataStore, ok bool) {
	s, ok = session.marketDataStores[symbol]
	return s, ok
}

func (session *ExchangeSession) StartPrice(symbol string) (price float64, ok bool) {
	price, ok = session.startPrices[symbol]
	return price, ok
}

func (session *ExchangeSession) LastPrice(symbol string) (price float64, ok bool) {
	price, ok = session.lastPrices[symbol]
	return price, ok
}

func (session *ExchangeSession) Market(symbol string) (market types.Market, ok bool) {
	market, ok = session.markets[symbol]
	return market, ok
}

func (session *ExchangeSession) OrderStore(symbol string) (store *OrderStore, ok bool) {
	store, ok = session.orderStores[symbol]
	return store, ok
}

// Subscribe save the subscription info, later it will be assigned to the stream
func (session *ExchangeSession) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) *ExchangeSession {
	sub := types.Subscription{
		Channel: channel,
		Symbol:  symbol,
		Options: options,
	}

	// add to the loaded symbol table
	session.usedSymbols[symbol] = struct{}{}
	session.Subscriptions[sub] = sub
	return session
}

func (session *ExchangeSession) FormatOrder(order types.SubmitOrder) (types.SubmitOrder, error) {
	market, ok := session.Market(order.Symbol)
	if !ok {
		return order, fmt.Errorf("market is not defined: %s", order.Symbol)
	}

	order.Market = market

	switch order.Type {
	case types.OrderTypeStopMarket, types.OrderTypeStopLimit:
		order.StopPriceString = market.FormatPrice(order.StopPrice)

	}

	switch order.Type {
	case types.OrderTypeMarket, types.OrderTypeStopMarket:
		order.Price = 0.0
		order.PriceString = ""

	default:
		order.PriceString = market.FormatPrice(order.Price)

	}

	order.QuantityString = market.FormatQuantity(order.Quantity)
	return order, nil
}

func (session *ExchangeSession) UpdatePrices(ctx context.Context) (err error) {
	if session.lastPriceUpdatedAt.After(time.Now().Add(- time.Hour)) {
		return nil
	}

	balances := session.Account.Balances()

	for _, b := range balances {
		priceSymbol := b.Currency + "USDT"
		startTime := time.Now().Add(-10 * time.Minute)
		klines, err := session.Exchange.QueryKLines(ctx, priceSymbol, types.Interval1m, types.KLineQueryOptions{
			Limit:     100,
			StartTime: &startTime,
		})

		if err != nil || len(klines) == 0 {
			continue
		}

		session.lastPrices[priceSymbol] = klines[len(klines)-1].Close
	}

	session.lastPriceUpdatedAt = time.Now()
	return err
}

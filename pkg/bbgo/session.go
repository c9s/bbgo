package bbgo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	log "github.com/sirupsen/logrus"
)

var (
	debugEWMA = false
	debugSMA  = false
)

func init() {
	// when using --dotenv option, the dotenv is loaded from command.PersistentPreRunE, not init.
	// hence here the env var won't enable the debug flag
	util.SetEnvVarBool("DEBUG_EWMA", &debugEWMA)
	util.SetEnvVarBool("DEBUG_SMA", &debugSMA)
}

type StandardIndicatorSet struct {
	Symbol string
	// Standard indicators
	// interval -> window
	sma   map[types.IntervalWindow]*indicator.SMA
	ewma  map[types.IntervalWindow]*indicator.EWMA
	boll  map[types.IntervalWindow]*indicator.BOLL
	stoch map[types.IntervalWindow]*indicator.STOCH

	store *MarketDataStore
}

func NewStandardIndicatorSet(symbol string, store *MarketDataStore) *StandardIndicatorSet {
	set := &StandardIndicatorSet{
		Symbol: symbol,
		sma:    make(map[types.IntervalWindow]*indicator.SMA),
		ewma:   make(map[types.IntervalWindow]*indicator.EWMA),
		boll:   make(map[types.IntervalWindow]*indicator.BOLL),
		stoch:  make(map[types.IntervalWindow]*indicator.STOCH),
		store:  store,
	}

	// let us pre-defined commonly used intervals
	for interval := range types.SupportedIntervals {
		for _, window := range []int{7, 25, 99} {
			iw := types.IntervalWindow{Interval: interval, Window: window}
			set.sma[iw] = &indicator.SMA{IntervalWindow: iw}
			set.sma[iw].Bind(store)
			if debugSMA {
				set.sma[iw].OnUpdate(func(value float64) {
					log.Infof("%s SMA %s: %f", symbol, iw.String(), value)
				})
			}

			set.ewma[iw] = &indicator.EWMA{IntervalWindow: iw}
			set.ewma[iw].Bind(store)

			// if debug EWMA is enabled, we add the debug handler
			if debugEWMA {
				set.ewma[iw].OnUpdate(func(value float64) {
					log.Infof("%s EWMA %s: %f", symbol, iw.String(), value)
				})
			}

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
		inc = &indicator.BOLL{IntervalWindow: iw, K: bandWidth}
		inc.Bind(set.store)
		set.boll[iw] = inc
	}

	return inc
}

// SMA returns the simple moving average indicator of the given interval and the window size.
func (set *StandardIndicatorSet) SMA(iw types.IntervalWindow) *indicator.SMA {
	inc, ok := set.sma[iw]
	if !ok {
		inc = &indicator.SMA{IntervalWindow: iw}
		inc.Bind(set.store)
		set.sma[iw] = inc
	}

	return inc
}

// EWMA returns the exponential weighed moving average indicator of the given interval and the window size.
func (set *StandardIndicatorSet) EWMA(iw types.IntervalWindow) *indicator.EWMA {
	inc, ok := set.ewma[iw]
	if !ok {
		inc = &indicator.EWMA{IntervalWindow: iw}
		inc.Bind(set.store)
		set.ewma[iw] = inc
	}

	return inc
}

func (set *StandardIndicatorSet) STOCH(iw types.IntervalWindow) *indicator.STOCH {
	inc, ok := set.stoch[iw]
	if !ok {
		inc = &indicator.STOCH{IntervalWindow: iw}
		inc.Bind(set.store)
		set.stoch[iw] = inc
	}

	return inc
}

// ExchangeSession presents the exchange connection Session
// It also maintains and collects the data returned from the stream.
type ExchangeSession struct {
	// exchange Session based notification system
	// we make it as a value field so that we can configure it separately
	Notifiability `json:"-" yaml:"-"`

	// ---------------------------
	// Session config fields
	// ---------------------------

	// Exchange Session name
	Name         string             `json:"name,omitempty" yaml:"name,omitempty"`
	ExchangeName types.ExchangeName `json:"exchange" yaml:"exchange"`
	EnvVarPrefix string             `json:"envVarPrefix" yaml:"envVarPrefix"`
	Key          string             `json:"key,omitempty" yaml:"key,omitempty"`
	Secret       string             `json:"secret,omitempty" yaml:"secret,omitempty"`
	SubAccount   string             `json:"subAccount,omitempty" yaml:"subAccount,omitempty"`

	// Withdrawal is used for enabling withdrawal functions
	Withdrawal   bool             `json:"withdrawal,omitempty" yaml:"withdrawal,omitempty"`
	MakerFeeRate fixedpoint.Value `json:"makerFeeRate,omitempty" yaml:"makerFeeRate,omitempty"`
	TakerFeeRate fixedpoint.Value `json:"takerFeeRate,omitempty" yaml:"takerFeeRate,omitempty"`

	PublicOnly           bool   `json:"publicOnly,omitempty" yaml:"publicOnly"`
	Margin               bool   `json:"margin,omitempty" yaml:"margin"`
	IsolatedMargin       bool   `json:"isolatedMargin,omitempty" yaml:"isolatedMargin,omitempty"`
	IsolatedMarginSymbol string `json:"isolatedMarginSymbol,omitempty" yaml:"isolatedMarginSymbol,omitempty"`
	Futures               bool   `json:"futures,omitempty" yaml:"futures"`
	IsolatedFutures      bool   `json:"isolatedFutures,omitempty" yaml:"isolatedFutures,omitempty"`
	IsolatedFuturesSymbol string `json:"isolatedFuturesSymbol,omitempty" yaml:"isolatedFuturesSymbol,omitempty"`
	

	// ---------------------------
	// Runtime fields
	// ---------------------------

	// The exchange account states
	Account *types.Account `json:"-" yaml:"-"`

	IsInitialized bool `json:"-" yaml:"-"`

	OrderExecutor *ExchangeOrderExecutor `json:"orderExecutor,omitempty" yaml:"orderExecutor,omitempty"`

	// UserDataStream is the connection stream of the exchange
	UserDataStream   types.Stream `json:"-" yaml:"-"`
	MarketDataStream types.Stream `json:"-" yaml:"-"`

	Subscriptions map[types.Subscription]types.Subscription `json:"-" yaml:"-"`

	Exchange types.Exchange `json:"-" yaml:"-"`

	// Trades collects the executed trades from the exchange
	// map: symbol -> []trade
	Trades map[string]*types.TradeSlice `json:"-" yaml:"-"`

	// markets defines market configuration of a symbol
	markets map[string]types.Market

	// orderBooks stores the streaming order book
	orderBooks map[string]*types.StreamOrderBook

	// startPrices is used for backtest
	startPrices map[string]float64

	lastPrices         map[string]float64
	lastPriceUpdatedAt time.Time

	// marketDataStores contains the market data store of each market
	marketDataStores map[string]*MarketDataStore

	positions map[string]*Position

	// standard indicators of each market
	standardIndicatorSets map[string]*StandardIndicatorSet

	orderStores map[string]*OrderStore

	usedSymbols        map[string]struct{}
	initializedSymbols map[string]struct{}

	logger *log.Entry
}

func NewExchangeSession(name string, exchange types.Exchange) *ExchangeSession {
	userDataStream := exchange.NewStream()
	marketDataStream := exchange.NewStream()
	marketDataStream.SetPublicOnly()

	session := &ExchangeSession{
		Notifiability: Notifiability{
			SymbolChannelRouter:  NewPatternChannelRouter(nil),
			SessionChannelRouter: NewPatternChannelRouter(nil),
			ObjectChannelRouter:  NewObjectChannelRouter(),
		},

		Name:             name,
		Exchange:         exchange,
		UserDataStream:   userDataStream,
		MarketDataStream: marketDataStream,
		Subscriptions:    make(map[types.Subscription]types.Subscription),
		Account:          &types.Account{},
		Trades:           make(map[string]*types.TradeSlice),

		orderBooks:            make(map[string]*types.StreamOrderBook),
		markets:               make(map[string]types.Market),
		startPrices:           make(map[string]float64),
		lastPrices:            make(map[string]float64),
		positions:             make(map[string]*Position),
		marketDataStores:      make(map[string]*MarketDataStore),
		standardIndicatorSets: make(map[string]*StandardIndicatorSet),
		orderStores:           make(map[string]*OrderStore),
		usedSymbols:           make(map[string]struct{}),
		initializedSymbols:    make(map[string]struct{}),
		logger:                log.WithField("session", name),
	}

	session.OrderExecutor = &ExchangeOrderExecutor{
		// copy the notification system so that we can route
		Notifiability: session.Notifiability,
		Session:       session,
	}

	return session
}

// Init initializes the basic data structure and market information by its exchange.
// Note that the subscribed symbols are not loaded in this stage.
func (session *ExchangeSession) Init(ctx context.Context, environ *Environment) error {
	if session.IsInitialized {
		return ErrSessionAlreadyInitialized
	}

	var log = log.WithField("session", session.Name)

	// load markets first

	var disableMarketsCache = false
	var markets types.MarketMap
	var err error
	if util.SetEnvVarBool("DISABLE_MARKETS_CACHE", &disableMarketsCache); disableMarketsCache {
		markets, err = session.Exchange.QueryMarkets(ctx)
	} else {
		markets, err = LoadExchangeMarketsWithCache(ctx, session.Exchange)
		if err != nil {
			return err
		}
	}

	if len(markets) == 0 {
		return fmt.Errorf("market config should not be empty")
	}

	session.markets = markets

	// query and initialize the balances
	log.Infof("querying balances from session %s...", session.Name)
	balances, err := session.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	log.Infof("%s account", session.Name)
	balances.Print()

	session.Account.UpdateBalances(balances)

	// forward trade updates and order updates to the order executor
	session.UserDataStream.OnTradeUpdate(session.OrderExecutor.EmitTradeUpdate)
	session.UserDataStream.OnOrderUpdate(session.OrderExecutor.EmitOrderUpdate)
	session.Account.BindStream(session.UserDataStream)

	// TODO: move this logic to Environment struct
	// if back-test service is not set, meaning we are not back-testing
	// we should insert trade into db right before everything
	if environ.BacktestService == nil {
		// if trade service is configured, we have the db configured
		if environ.TradeService != nil {
			session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
				if err := environ.TradeService.Insert(trade); err != nil {
					log.WithError(err).Errorf("trade insert error: %+v", trade)
				}
			})
		}
	}

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		log.WithField("marketData", "kline").Infof("kline closed: %+v", kline)
	})

	// update last prices
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if _, ok := session.startPrices[kline.Symbol]; !ok {
			session.startPrices[kline.Symbol] = kline.Open
		}

		session.lastPrices[kline.Symbol] = kline.Close
	})

	session.IsInitialized = true
	return nil
}

func (session *ExchangeSession) InitSymbols(ctx context.Context, environ *Environment) error {
	if err := session.initUsedSymbols(ctx, environ); err != nil {
		return err
	}

	return nil
}

// initUsedSymbols uses usedSymbols to initialize the related data structure
func (session *ExchangeSession) initUsedSymbols(ctx context.Context, environ *Environment) error {
	for symbol := range session.usedSymbols {
		if err := session.initSymbol(ctx, environ, symbol); err != nil {
			return err
		}
	}

	return nil
}

// initSymbol loads trades for the symbol, bind stream callbacks, init positions, market data store.
// please note, initSymbol can not be called for the same symbol for twice
func (session *ExchangeSession) initSymbol(ctx context.Context, environ *Environment, symbol string) error {
	if _, ok := session.initializedSymbols[symbol]; ok {
		// return fmt.Errorf("symbol %s is already initialized", symbol)
		return nil
	}

	market, ok := session.markets[symbol]
	if !ok {
		return fmt.Errorf("market %s is not defined", symbol)
	}

	var err error
	var trades []types.Trade
	if environ.SyncService != nil {
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
	session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		session.Trades[symbol].Append(trade)
	})

	position := &Position{
		Symbol:        symbol,
		BaseCurrency:  market.BaseCurrency,
		QuoteCurrency: market.QuoteCurrency,
	}
	position.AddTrades(trades)
	position.BindStream(session.UserDataStream)
	session.positions[symbol] = position

	orderStore := NewOrderStore(symbol)
	orderStore.AddOrderUpdate = true

	orderStore.BindStream(session.UserDataStream)
	session.orderStores[symbol] = orderStore

	marketDataStore := NewMarketDataStore(symbol)
	marketDataStore.BindStream(session.MarketDataStream)
	session.marketDataStores[symbol] = marketDataStore

	standardIndicatorSet := NewStandardIndicatorSet(symbol, marketDataStore)
	session.standardIndicatorSets[symbol] = standardIndicatorSet

	// used kline intervals by the given symbol
	var klineSubscriptions = map[types.Interval]struct{}{}

	// always subscribe the 1m kline so we can make sure the connection persists.
	klineSubscriptions[types.Interval1m] = struct{}{}

	// Aggregate the intervals that we are using in the subscriptions.
	for _, sub := range session.Subscriptions {
		switch sub.Channel {
		case types.BookChannel:
			book := types.NewStreamBook(sub.Symbol)
			book.BindStream(session.MarketDataStream)
			session.orderBooks[sub.Symbol] = book

		case types.KLineChannel:
			if sub.Options.Interval == "" {
				continue
			}

			if sub.Symbol == symbol {
				klineSubscriptions[types.Interval(sub.Options.Interval)] = struct{}{}
			}
		}
	}

	for interval := range klineSubscriptions {
		// avoid querying the last unclosed kline
		endTime := environ.startTime
		kLines, err := session.Exchange.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
			EndTime: &endTime,
			Limit:   1000, // indicators need at least 100
		})
		if err != nil {
			return err
		}

		if len(kLines) == 0 {
			log.Warnf("no kline data for %s %s (end time <= %s)", symbol, interval, environ.startTime)
			continue
		}

		// update last prices by the given kline
		lastKLine := kLines[len(kLines)-1]
		if interval == types.Interval1m {
			log.Infof("last kline %+v", lastKLine)
			session.lastPrices[symbol] = lastKLine.Close
		}

		for _, k := range kLines {
			// let market data store trigger the update, so that the indicator could be updated too.
			marketDataStore.AddKLine(k)
		}
	}

	log.Infof("%s last price: %f", symbol, session.lastPrices[symbol])

	session.initializedSymbols[symbol] = struct{}{}
	return nil
}

func (session *ExchangeSession) StandardIndicatorSet(symbol string) (*StandardIndicatorSet, bool) {
	set, ok := session.standardIndicatorSets[symbol]
	return set, ok
}

func (session *ExchangeSession) Position(symbol string) (pos *Position, ok bool) {
	pos, ok = session.positions[symbol]
	if ok {
		return pos, ok
	}

	market, ok := session.markets[symbol]
	if !ok {
		return nil, false
	}

	pos = &Position{
		Symbol:        symbol,
		BaseCurrency:  market.BaseCurrency,
		QuoteCurrency: market.QuoteCurrency,
	}
	ok = true
	session.positions[symbol] = pos
	return pos, ok
}

func (session *ExchangeSession) Positions() map[string]*Position {
	return session.positions
}

// MarketDataStore returns the market data store of a symbol
func (session *ExchangeSession) MarketDataStore(symbol string) (s *MarketDataStore, ok bool) {
	s, ok = session.marketDataStores[symbol]
	return s, ok
}

// MarketDataStore returns the market data store of a symbol
func (session *ExchangeSession) OrderBook(symbol string) (s *types.StreamOrderBook, ok bool) {
	s, ok = session.orderBooks[symbol]
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

func (session *ExchangeSession) LastPrices() map[string]float64 {
	return session.lastPrices
}

func (session *ExchangeSession) Market(symbol string) (market types.Market, ok bool) {
	market, ok = session.markets[symbol]
	return market, ok
}

func (session *ExchangeSession) Markets() map[string]types.Market {
	return session.markets
}

func (session *ExchangeSession) OrderStore(symbol string) (store *OrderStore, ok bool) {
	store, ok = session.orderStores[symbol]
	return store, ok
}

func (session *ExchangeSession) OrderStores() map[string]*OrderStore {
	return session.orderStores
}

// Subscribe save the subscription info, later it will be assigned to the stream
func (session *ExchangeSession) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) *ExchangeSession {
	if channel == types.KLineChannel && len(options.Interval) == 0 {
		panic("subscription interval for kline can not be empty")
	}

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
	if session.lastPriceUpdatedAt.After(time.Now().Add(-time.Hour)) {
		return nil
	}

	balances := session.Account.Balances()

	symbols := make([]string, len(balances))
	for _, b := range balances {
		symbols = append(symbols, b.Currency+"USDT")
	}

	tickers, err := session.Exchange.QueryTickers(ctx, symbols...)

	if err != nil || len(tickers) == 0 {
		return err
	}

	for k, v := range tickers {
		session.lastPrices[k] = v.Last
	}

	session.lastPriceUpdatedAt = time.Now()
	return err
}

func (session *ExchangeSession) FindPossibleSymbols() (symbols []string, err error) {
	// If the session is an isolated margin session, there will be only the isolated margin symbol
	if session.Margin && session.IsolatedMargin {
		return []string{
			session.IsolatedMarginSymbol,
		}, nil
	}

	var balances = session.Account.Balances()
	var fiatAssets []string

	for _, currency := range types.FiatCurrencies {
		if balance, ok := balances[currency]; ok && balance.Total() > 0 {
			fiatAssets = append(fiatAssets, currency)
		}
	}

	var symbolMap = map[string]struct{}{}

	for _, market := range session.Markets() {
		// ignore the markets that are not fiat currency markets
		if !util.StringSliceContains(fiatAssets, market.QuoteCurrency) {
			continue
		}

		// ignore the asset that we don't have in the balance sheet
		balance, hasAsset := balances[market.BaseCurrency]
		if !hasAsset || balance.Total() == 0 {
			continue
		}

		symbolMap[market.Symbol] = struct{}{}
	}

	for s := range symbolMap {
		symbols = append(symbols, s)
	}

	return symbols, nil
}

func InitExchangeSession(name string, session *ExchangeSession) error {
	var err error
	var exchangeName = session.ExchangeName
	var exchange types.Exchange
	if session.Key != "" && session.Secret != "" {
		if !session.PublicOnly {
			if len(session.Key) == 0 || len(session.Secret) == 0 {
				return fmt.Errorf("can not create exchange %s: empty key or secret", exchangeName)
			}
		}

		exchange, err = cmdutil.NewExchangeStandard(exchangeName, session.Key, session.Secret, "", session.SubAccount)
	} else {
		exchange, err = cmdutil.NewExchangeWithEnvVarPrefix(exchangeName, session.EnvVarPrefix)
	}

	if err != nil {
		return err
	}

	// configure exchange
	if session.Margin {
		marginExchange, ok := exchange.(types.MarginExchange)
		if !ok {
			return fmt.Errorf("exchange %s does not support margin", exchangeName)
		}

		if session.IsolatedMargin {
			marginExchange.UseIsolatedMargin(session.IsolatedMarginSymbol)
		} else {
			marginExchange.UseMargin()
		}
	}

	if session.Futures {
		futuresExchange, ok := exchange.(types.FuturesExchange)
		if !ok {
			return fmt.Errorf("exchange %s does not support futures", exchangeName)
		}

		if session.IsolatedFutures {
			futuresExchange.UseIsolatedFutures(session.IsolatedFuturesSymbol)
		} else {
			futuresExchange.UseFutures()
		}
	}

	session.Name = name
	session.Notifiability = Notifiability{
		SymbolChannelRouter:  NewPatternChannelRouter(nil),
		SessionChannelRouter: NewPatternChannelRouter(nil),
		ObjectChannelRouter:  NewObjectChannelRouter(),
	}
	session.Exchange = exchange
	session.UserDataStream = exchange.NewStream()
	session.MarketDataStream = exchange.NewStream()
	session.MarketDataStream.SetPublicOnly()

	// pointer fields
	session.Subscriptions = make(map[types.Subscription]types.Subscription)
	session.Account = &types.Account{}
	session.Trades = make(map[string]*types.TradeSlice)

	session.orderBooks = make(map[string]*types.StreamOrderBook)
	session.markets = make(map[string]types.Market)
	session.lastPrices = make(map[string]float64)
	session.startPrices = make(map[string]float64)
	session.marketDataStores = make(map[string]*MarketDataStore)
	session.positions = make(map[string]*Position)
	session.standardIndicatorSets = make(map[string]*StandardIndicatorSet)
	session.orderStores = make(map[string]*OrderStore)
	session.OrderExecutor = &ExchangeOrderExecutor{
		// copy the notification system so that we can route
		Notifiability: session.Notifiability,
		Session:       session,
	}

	session.usedSymbols = make(map[string]struct{})
	session.initializedSymbols = make(map[string]struct{})
	session.logger = log.WithField("session", name)
	return nil
}

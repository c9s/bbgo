package bbgo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/cache"

	exchange2 "github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
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
	sma        map[types.IntervalWindow]*indicator.SMA
	ewma       map[types.IntervalWindow]*indicator.EWMA
	boll       map[types.IntervalWindowBandWidth]*indicator.BOLL
	stoch      map[types.IntervalWindow]*indicator.STOCH
	volatility map[types.IntervalWindow]*indicator.VOLATILITY

	store *MarketDataStore
}

func NewStandardIndicatorSet(symbol string, store *MarketDataStore) *StandardIndicatorSet {
	set := &StandardIndicatorSet{
		Symbol:     symbol,
		sma:        make(map[types.IntervalWindow]*indicator.SMA),
		ewma:       make(map[types.IntervalWindow]*indicator.EWMA),
		boll:       make(map[types.IntervalWindowBandWidth]*indicator.BOLL),
		stoch:      make(map[types.IntervalWindow]*indicator.STOCH),
		volatility: make(map[types.IntervalWindow]*indicator.VOLATILITY),
		store:      store,
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

		// set efault band width to 2.0
		iwb := types.IntervalWindowBandWidth{IntervalWindow: iw, BandWidth: 2.0}
		set.boll[iwb] = &indicator.BOLL{IntervalWindow: iw, K: iwb.BandWidth}
		set.boll[iwb].Bind(store)
	}

	return set
}

// BOLL returns the bollinger band indicator of the given interval, the window and bandwidth
func (set *StandardIndicatorSet) BOLL(iw types.IntervalWindow, bandWidth float64) *indicator.BOLL {
	iwb := types.IntervalWindowBandWidth{IntervalWindow: iw, BandWidth: bandWidth}
	inc, ok := set.boll[iwb]
	if !ok {
		inc = &indicator.BOLL{IntervalWindow: iw, K: bandWidth}
		inc.Bind(set.store)
		set.boll[iwb] = inc
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

// VOLATILITY returns the volatility(stddev) indicator of the given interval and the window size.
func (set *StandardIndicatorSet) VOLATILITY(iw types.IntervalWindow) *indicator.VOLATILITY {
	inc, ok := set.volatility[iw]
	if !ok {
		inc = &indicator.VOLATILITY{IntervalWindow: iw}
		inc.Bind(set.store)
		set.volatility[iw] = inc
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
	Passphrase   string             `json:"passphrase,omitempty" yaml:"passphrase,omitempty"`
	SubAccount   string             `json:"subAccount,omitempty" yaml:"subAccount,omitempty"`

	// Withdrawal is used for enabling withdrawal functions
	Withdrawal   bool             `json:"withdrawal,omitempty" yaml:"withdrawal,omitempty"`
	MakerFeeRate fixedpoint.Value `json:"makerFeeRate" yaml:"makerFeeRate"`
	TakerFeeRate fixedpoint.Value `json:"takerFeeRate" yaml:"takerFeeRate"`

	PublicOnly           bool   `json:"publicOnly,omitempty" yaml:"publicOnly"`
	Margin               bool   `json:"margin,omitempty" yaml:"margin"`
	IsolatedMargin       bool   `json:"isolatedMargin,omitempty" yaml:"isolatedMargin,omitempty"`
	IsolatedMarginSymbol string `json:"isolatedMarginSymbol,omitempty" yaml:"isolatedMarginSymbol,omitempty"`

	Futures               bool   `json:"futures,omitempty" yaml:"futures"`
	IsolatedFutures       bool   `json:"isolatedFutures,omitempty" yaml:"isolatedFutures,omitempty"`
	IsolatedFuturesSymbol string `json:"isolatedFuturesSymbol,omitempty" yaml:"isolatedFuturesSymbol,omitempty"`

	// ---------------------------
	// Runtime fields
	// ---------------------------

	// The exchange account states
	Account      *types.Account `json:"-" yaml:"-"`
	accountMutex sync.Mutex

	IsInitialized bool `json:"-" yaml:"-"`

	OrderExecutor *ExchangeOrderExecutor `json:"orderExecutor,omitempty" yaml:"orderExecutor,omitempty"`

	// UserDataStream is the connection stream of the exchange
	UserDataStream   types.Stream `json:"-" yaml:"-"`
	MarketDataStream types.Stream `json:"-" yaml:"-"`

	// Subscriptions
	// this is a read-only field when running strategy
	Subscriptions map[types.Subscription]types.Subscription `json:"-" yaml:"-"`

	Exchange types.Exchange `json:"-" yaml:"-"`

	UseHeikinAshi bool `json:"heikinAshi,omitempty" yaml:"heikinAshi,omitempty"`

	// Trades collects the executed trades from the exchange
	// map: symbol -> []trade
	Trades map[string]*types.TradeSlice `json:"-" yaml:"-"`

	// markets defines market configuration of a symbol
	markets map[string]types.Market

	// orderBooks stores the streaming order book
	orderBooks map[string]*types.StreamOrderBook

	// startPrices is used for backtest
	startPrices map[string]fixedpoint.Value

	lastPrices         map[string]fixedpoint.Value
	lastPriceUpdatedAt time.Time

	// marketDataStores contains the market data store of each market
	marketDataStores map[string]*MarketDataStore

	positions map[string]*types.Position

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
		startPrices:           make(map[string]fixedpoint.Value),
		lastPrices:            make(map[string]fixedpoint.Value),
		positions:             make(map[string]*types.Position),
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

func (session *ExchangeSession) GetAccount() (a *types.Account) {
	session.accountMutex.Lock()
	a = session.Account
	session.accountMutex.Unlock()
	return a
}

// UpdateAccount locks the account mutex and update the account object
func (session *ExchangeSession) UpdateAccount(ctx context.Context) (*types.Account, error) {
	account, err := session.Exchange.QueryAccount(ctx)
	if err != nil {
		return nil, err
	}

	session.accountMutex.Lock()
	session.Account = account
	session.accountMutex.Unlock()
	return account, nil
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
		markets, err = cache.LoadExchangeMarketsWithCache(ctx, session.Exchange)
		if err != nil {
			return err
		}
	}

	if len(markets) == 0 {
		return fmt.Errorf("market config should not be empty")
	}

	session.markets = markets

	if feeRateProvider, ok := session.Exchange.(types.ExchangeDefaultFeeRates); ok {
		defaultFeeRates := feeRateProvider.DefaultFeeRates()
		if session.MakerFeeRate.IsZero() {
			session.MakerFeeRate = defaultFeeRates.MakerFeeRate
		}
		if session.TakerFeeRate.IsZero() {
			session.TakerFeeRate = defaultFeeRates.TakerFeeRate
		}
	}

	if session.UseHeikinAshi {
		session.MarketDataStream = &types.HeikinAshiStream{
			StandardStreamEmitter: session.MarketDataStream.(types.StandardStreamEmitter),
		}
	}

	// query and initialize the balances
	if !session.PublicOnly {
		account, err := session.Exchange.QueryAccount(ctx)
		if err != nil {
			return err
		}

		session.accountMutex.Lock()
		session.Account = account
		session.accountMutex.Unlock()

		log.Infof("%s account", session.Name)
		account.Balances().Print()

		// forward trade updates and order updates to the order executor
		session.UserDataStream.OnTradeUpdate(session.OrderExecutor.EmitTradeUpdate)
		session.UserDataStream.OnOrderUpdate(session.OrderExecutor.EmitOrderUpdate)

		session.UserDataStream.OnBalanceSnapshot(func(balances types.BalanceMap) {
			session.accountMutex.Lock()
			session.Account.UpdateBalances(balances)
			session.accountMutex.Unlock()
		})

		session.UserDataStream.OnBalanceUpdate(func(balances types.BalanceMap) {
			session.accountMutex.Lock()
			session.Account.UpdateBalances(balances)
			session.accountMutex.Unlock()
		})

		session.bindConnectionStatusNotification(session.UserDataStream, "user data")

		// if metrics mode is enabled, we bind the callbacks to update metrics
		if viper.GetBool("metrics") {
			session.metricsBalancesUpdater(account.Balances())
			session.bindUserDataStreamMetrics(session.UserDataStream)
		}
	}

	// add trade logger
	session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		log.Info(trade.String())
	})

	if viper.GetBool("debug-kline") {
		session.MarketDataStream.OnKLine(func(kline types.KLine) {
			log.WithField("marketData", "kline").Infof("kline: %+v", kline)
		})
		session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
			log.WithField("marketData", "kline").Infof("kline closed: %+v", kline)
		})
	}

	// update last prices
	if session.UseHeikinAshi {
		session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
			if _, ok := session.startPrices[kline.Symbol]; !ok {
				session.startPrices[kline.Symbol] = kline.Open
			}

			session.lastPrices[kline.Symbol] = session.MarketDataStream.(*types.HeikinAshiStream).LastOrigin[kline.Symbol][kline.Interval].Close
		})
	} else {
		session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
			if _, ok := session.startPrices[kline.Symbol]; !ok {
				session.startPrices[kline.Symbol] = kline.Open
			}

			session.lastPrices[kline.Symbol] = kline.Close
		})
	}

	session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {
		session.lastPrices[trade.Symbol] = trade.Price
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
	if environ.SyncService != nil && environ.BacktestService == nil {
		tradingFeeCurrency := session.Exchange.PlatformFeeCurrency()
		if strings.HasPrefix(symbol, tradingFeeCurrency) {
			trades, err = environ.TradeService.QueryForTradingFeeCurrency(session.Exchange.Name(), symbol, tradingFeeCurrency)
		} else {
			trades, err = environ.TradeService.Query(service.QueryTradesOptions{
				Exchange: session.Exchange.Name(),
				Symbol:   symbol,
				Ordering: "DESC",
				Limit:    100,
			})
		}

		if err != nil {
			return err
		}

		trades = types.SortTradesAscending(trades)
		log.Infof("symbol %s: %d trades loaded", symbol, len(trades))
	}

	session.Trades[symbol] = &types.TradeSlice{Trades: trades}
	session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		if trade.Symbol == symbol {
			session.Trades[symbol].Append(trade)
		}
	})

	position := &types.Position{
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

	log.Infof("%s last price: %v", symbol, session.lastPrices[symbol])

	session.initializedSymbols[symbol] = struct{}{}
	return nil
}

func (session *ExchangeSession) StandardIndicatorSet(symbol string) (*StandardIndicatorSet, bool) {
	set, ok := session.standardIndicatorSets[symbol]
	return set, ok
}

func (session *ExchangeSession) Position(symbol string) (pos *types.Position, ok bool) {
	pos, ok = session.positions[symbol]
	if ok {
		return pos, ok
	}

	market, ok := session.markets[symbol]
	if !ok {
		return nil, false
	}

	pos = &types.Position{
		Symbol:        symbol,
		BaseCurrency:  market.BaseCurrency,
		QuoteCurrency: market.QuoteCurrency,
	}
	ok = true
	session.positions[symbol] = pos
	return pos, ok
}

func (session *ExchangeSession) Positions() map[string]*types.Position {
	return session.positions
}

// MarketDataStore returns the market data store of a symbol
func (session *ExchangeSession) MarketDataStore(symbol string) (s *MarketDataStore, ok bool) {
	s, ok = session.marketDataStores[symbol]
	return s, ok
}

// OrderBook returns the personal orderbook of a symbol
func (session *ExchangeSession) OrderBook(symbol string) (s *types.StreamOrderBook, ok bool) {
	s, ok = session.orderBooks[symbol]
	return s, ok
}

func (session *ExchangeSession) StartPrice(symbol string) (price fixedpoint.Value, ok bool) {
	price, ok = session.startPrices[symbol]
	return price, ok
}

func (session *ExchangeSession) LastPrice(symbol string) (price fixedpoint.Value, ok bool) {
	price, ok = session.lastPrices[symbol]
	return price, ok
}

func (session *ExchangeSession) AllLastPrices() map[string]fixedpoint.Value {
	return session.lastPrices
}

func (session *ExchangeSession) LastPrices() map[string]fixedpoint.Value {
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
	return order, nil
}

func (session *ExchangeSession) UpdatePrices(ctx context.Context, currencies []string, fiat string) (err error) {
	// TODO: move this cache check to the http routes
	// if session.lastPriceUpdatedAt.After(time.Now().Add(-time.Hour)) {
	// 	return nil
	// }

	var symbols []string
	for _, c := range currencies {
		symbols = append(symbols, c+fiat) // BTC/USDT
		symbols = append(symbols, fiat+c) // USDT/TWD
	}

	tickers, err := session.Exchange.QueryTickers(ctx, symbols...)
	if err != nil || len(tickers) == 0 {
		return err
	}

	var lastTime time.Time
	for k, v := range tickers {
		// for {Crypto}/USDT markets
		session.lastPrices[k] = v.Last
		if v.Time.After(lastTime) {
			lastTime = v.Time
		}
	}

	session.lastPriceUpdatedAt = lastTime
	return err
}

func (session *ExchangeSession) FindPossibleSymbols() (symbols []string, err error) {
	// If the session is an isolated margin session, there will be only the isolated margin symbol
	if session.Margin && session.IsolatedMargin {
		return []string{
			session.IsolatedMarginSymbol,
		}, nil
	}

	var balances = session.GetAccount().Balances()
	var fiatAssets []string

	for _, currency := range types.FiatCurrencies {
		if balance, ok := balances[currency]; ok && balance.Total().Sign() > 0 {
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
		if !hasAsset || balance.Total().IsZero() {
			continue
		}

		symbolMap[market.Symbol] = struct{}{}
	}

	for s := range symbolMap {
		symbols = append(symbols, s)
	}

	return symbols, nil
}

// InitExchange initialize the exchange instance and allocate memory for fields
// In this stage, the session var could be loaded from the JSON config, so the pointer fields are still nil
// The Init method will be called after this stage, environment.Init will call the session.Init method later.
func (session *ExchangeSession) InitExchange(name string, ex types.Exchange) error {
	var err error
	var exchangeName = session.ExchangeName
	if ex == nil {
		if session.PublicOnly {
			ex, err = exchange2.NewPublic(exchangeName)
		} else {
			if session.Key != "" && session.Secret != "" {
				ex, err = exchange2.NewStandard(exchangeName, session.Key, session.Secret, session.Passphrase, session.SubAccount)
			} else {
				ex, err = exchange2.NewWithEnvVarPrefix(exchangeName, session.EnvVarPrefix)
			}
		}
	}

	if err != nil {
		return err
	}

	// configure exchange
	if session.Margin {
		marginExchange, ok := ex.(types.MarginExchange)
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
		futuresExchange, ok := ex.(types.FuturesExchange)
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
	session.Exchange = ex
	session.UserDataStream = ex.NewStream()
	session.MarketDataStream = ex.NewStream()
	session.MarketDataStream.SetPublicOnly()

	// pointer fields
	session.Subscriptions = make(map[types.Subscription]types.Subscription)
	session.Account = &types.Account{}
	session.Trades = make(map[string]*types.TradeSlice)

	session.orderBooks = make(map[string]*types.StreamOrderBook)
	session.markets = make(map[string]types.Market)
	session.lastPrices = make(map[string]fixedpoint.Value)
	session.startPrices = make(map[string]fixedpoint.Value)
	session.marketDataStores = make(map[string]*MarketDataStore)
	session.positions = make(map[string]*types.Position)
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

func (session *ExchangeSession) MarginType() string {
	margin := "none"
	if session.Margin {
		margin = "margin"
		if session.IsolatedMargin {
			margin = "isolated"
		}
	}
	return margin
}

func (session *ExchangeSession) metricsBalancesUpdater(balances types.BalanceMap) {
	for currency, balance := range balances {
		labels := prometheus.Labels{
			"exchange": session.ExchangeName.String(),
			"margin":   session.MarginType(),
			"symbol":   session.IsolatedMarginSymbol,
			"currency": currency,
		}

		metricsTotalBalances.With(labels).Set(balance.Total().Float64())
		metricsLockedBalances.With(labels).Set(balance.Locked.Float64())
		metricsAvailableBalances.With(labels).Set(balance.Available.Float64())
		metricsLastUpdateTimeBalance.With(prometheus.Labels{
			"exchange":  session.ExchangeName.String(),
			"margin":    session.MarginType(),
			"channel":   "user",
			"data_type": "balance",
			"symbol":    "",
			"currency":  currency,
		}).SetToCurrentTime()
	}

}

func (session *ExchangeSession) metricsOrderUpdater(order types.Order) {
	metricsLastUpdateTimeBalance.With(prometheus.Labels{
		"exchange":  session.ExchangeName.String(),
		"margin":    session.MarginType(),
		"channel":   "user",
		"data_type": "order",
		"symbol":    order.Symbol,
		"currency":  "",
	}).SetToCurrentTime()
}

func (session *ExchangeSession) metricsTradeUpdater(trade types.Trade) {
	labels := prometheus.Labels{
		"exchange":  session.ExchangeName.String(),
		"margin":    session.MarginType(),
		"side":      trade.Side.String(),
		"symbol":    trade.Symbol,
		"liquidity": trade.Liquidity(),
	}
	metricsTradingVolume.With(labels).Add(trade.Quantity.Mul(trade.Price).Float64())
	metricsTradesTotal.With(labels).Inc()
	metricsLastUpdateTimeBalance.With(prometheus.Labels{
		"exchange":  session.ExchangeName.String(),
		"margin":    session.MarginType(),
		"channel":   "user",
		"data_type": "trade",
		"symbol":    trade.Symbol,
		"currency":  "",
	}).SetToCurrentTime()
}

func (session *ExchangeSession) bindMarketDataStreamMetrics(stream types.Stream) {
	stream.OnBookUpdate(func(book types.SliceOrderBook) {
		metricsLastUpdateTimeBalance.With(prometheus.Labels{
			"exchange":  session.ExchangeName.String(),
			"margin":    session.MarginType(),
			"channel":   "market",
			"data_type": "book",
			"symbol":    book.Symbol,
			"currency":  "",
		}).SetToCurrentTime()
	})
	stream.OnKLineClosed(func(kline types.KLine) {
		metricsLastUpdateTimeBalance.With(prometheus.Labels{
			"exchange":  session.ExchangeName.String(),
			"margin":    session.MarginType(),
			"channel":   "market",
			"data_type": "kline",
			"symbol":    kline.Symbol,
			"currency":  "",
		}).SetToCurrentTime()
	})
}

func (session *ExchangeSession) bindUserDataStreamMetrics(stream types.Stream) {
	stream.OnBalanceUpdate(session.metricsBalancesUpdater)
	stream.OnBalanceSnapshot(session.metricsBalancesUpdater)
	stream.OnTradeUpdate(session.metricsTradeUpdater)
	stream.OnOrderUpdate(session.metricsOrderUpdater)
	stream.OnDisconnect(func() {
		metricsConnectionStatus.With(prometheus.Labels{
			"channel":  "user",
			"exchange": session.ExchangeName.String(),
			"margin":   session.MarginType(),
			"symbol":   session.IsolatedMarginSymbol,
		}).Set(0.0)
	})
	stream.OnConnect(func() {
		metricsConnectionStatus.With(prometheus.Labels{
			"channel":  "user",
			"exchange": session.ExchangeName.String(),
			"margin":   session.MarginType(),
			"symbol":   session.IsolatedMarginSymbol,
		}).Set(1.0)
	})
}

func (session *ExchangeSession) bindConnectionStatusNotification(stream types.Stream, streamName string) {
	stream.OnDisconnect(func() {
		Notify("session %s %s stream disconnected", session.Name, streamName)
	})
	stream.OnConnect(func() {
		Notify("session %s %s stream connected", session.Name, streamName)
	})
}

func (session *ExchangeSession) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField
	var footerIcon = types.ExchangeFooterIcon(session.ExchangeName)
	return slack.Attachment{
		// Pretext:       "",
		// Text:  text,
		Title:      session.Name,
		Fields:     fields,
		FooterIcon: footerIcon,
		Footer:     util.Render("update time {{ . }}", time.Now().Format(time.RFC822)),
	}
}

func (session *ExchangeSession) FormatOrders(orders []types.SubmitOrder) (formattedOrders []types.SubmitOrder, err error) {
	for _, order := range orders {
		o, err := session.FormatOrder(order)
		if err != nil {
			return formattedOrders, err
		}
		formattedOrders = append(formattedOrders, o)
	}

	return formattedOrders, err
}

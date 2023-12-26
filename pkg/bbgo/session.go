package bbgo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/slack-go/slack"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/cache"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/util/templateutil"

	exchange2 "github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var KLinePreloadLimit int64 = 1000

var ErrEmptyMarketInfo = errors.New("market info should not be empty, 0 markets loaded")

// ExchangeSession presents the exchange connection Session
// It also maintains and collects the data returned from the stream.
type ExchangeSession struct {
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
	Withdrawal              bool             `json:"withdrawal,omitempty" yaml:"withdrawal,omitempty"`
	MakerFeeRate            fixedpoint.Value `json:"makerFeeRate" yaml:"makerFeeRate"`
	TakerFeeRate            fixedpoint.Value `json:"takerFeeRate" yaml:"takerFeeRate"`
	ModifyOrderAmountForFee bool             `json:"modifyOrderAmountForFee" yaml:"modifyOrderAmountForFee"`

	// PublicOnly is used for setting the session to public only (without authentication, no private user data)
	PublicOnly bool `json:"publicOnly,omitempty" yaml:"publicOnly"`

	// PrivateChannels is used for filtering the private user data channel, .e.g, orders, trades, balances.. etc
	// This option is exchange specific
	PrivateChannels []string `json:"privateChannels,omitempty" yaml:"privateChannels,omitempty"`

	// PrivateChannelSymbols is used for filtering the private user data channel, .e.g, order symbol subscription.
	// This option is exchange specific
	PrivateChannelSymbols []string `json:"privateChannelSymbols,omitempty" yaml:"privateChannelSymbols,omitempty"`

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

	// indicators is the v2 api indicators
	indicators map[string]*IndicatorSet

	orderStores map[string]*core.OrderStore

	usedSymbols        map[string]struct{}
	initializedSymbols map[string]struct{}

	logger log.FieldLogger
}

func NewExchangeSession(name string, exchange types.Exchange) *ExchangeSession {
	userDataStream := exchange.NewStream()
	marketDataStream := exchange.NewStream()
	marketDataStream.SetPublicOnly()

	session := &ExchangeSession{
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
		indicators:            make(map[string]*IndicatorSet),
		orderStores:           make(map[string]*core.OrderStore),
		usedSymbols:           make(map[string]struct{}),
		initializedSymbols:    make(map[string]struct{}),
		logger:                log.WithField("session", name),
	}

	session.OrderExecutor = &ExchangeOrderExecutor{
		// copy the notification system so that we can route
		Session: session,
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

	session.setAccount(account)
	return account, nil
}

func (session *ExchangeSession) setAccount(a *types.Account) {
	session.accountMutex.Lock()
	session.Account = a
	session.accountMutex.Unlock()
}

// Init initializes the basic data structure and market information by its exchange.
// Note that the subscribed symbols are not loaded in this stage.
func (session *ExchangeSession) Init(ctx context.Context, environ *Environment) error {
	if session.IsInitialized {
		return ErrSessionAlreadyInitialized
	}

	var logger = environ.Logger()
	logger = logger.WithField("session", session.Name)

	// override the default logger
	session.logger = logger

	// load markets first
	logger.Infof("querying market info from %s...", session.Name)

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
		return ErrEmptyMarketInfo
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

	if session.ModifyOrderAmountForFee {
		amountProtectExchange, ok := session.Exchange.(types.ExchangeAmountFeeProtect)
		if !ok {
			return fmt.Errorf("exchange %s does not support order amount protection", session.ExchangeName.String())
		}

		fees := types.ExchangeFee{MakerFeeRate: session.MakerFeeRate, TakerFeeRate: session.TakerFeeRate}
		amountProtectExchange.SetModifyOrderAmountForFee(fees)
	}

	if session.UseHeikinAshi {
		session.MarketDataStream = &types.HeikinAshiStream{
			StandardStreamEmitter: session.MarketDataStream.(types.StandardStreamEmitter),
		}
	}

	// query and initialize the balances
	if !session.PublicOnly {
		if len(session.PrivateChannels) > 0 {
			if setter, ok := session.UserDataStream.(types.PrivateChannelSetter); ok {
				setter.SetPrivateChannels(session.PrivateChannels)
			}
		}
		if len(session.PrivateChannelSymbols) > 0 {
			if setter, ok := session.UserDataStream.(types.PrivateChannelSymbolSetter); ok {
				setter.SetPrivateChannelSymbols(session.PrivateChannelSymbols)
			}
		}

		disableStartupBalanceQuery := environ.environmentConfig != nil && environ.environmentConfig.DisableStartupBalanceQuery
		if disableStartupBalanceQuery {
			session.accountMutex.Lock()
			session.Account = types.NewAccount()
			session.accountMutex.Unlock()
		} else {
			logger.Infof("querying account balances...")
			account, err := retry.QueryAccountUntilSuccessful(ctx, session.Exchange)
			if err != nil {
				return err
			}

			session.setAccount(account)
			session.metricsBalancesUpdater(account.Balances())
			logger.Infof("account %s balances:\n%s", session.Name, account.Balances().String())
		}

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
			session.bindUserDataStreamMetrics(session.UserDataStream)
		}
	}

	if environ.loggingConfig != nil {
		if environ.loggingConfig.Balance {
			session.UserDataStream.OnBalanceSnapshot(func(balances types.BalanceMap) {
				logger.Info(balances.String())
			})
			session.UserDataStream.OnBalanceUpdate(func(balances types.BalanceMap) {
				logger.Info(balances.String())
			})
		}

		if environ.loggingConfig.Trade {
			session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
				logger.Info(trade.String())
			})
		}

		if environ.loggingConfig.FilledOrderOnly {
			session.UserDataStream.OnOrderUpdate(func(order types.Order) {
				if order.Status == types.OrderStatusFilled {
					logger.Info(order.String())
				}
			})
		} else if environ.loggingConfig.Order {
			session.UserDataStream.OnOrderUpdate(func(order types.Order) {
				logger.Info(order.String())
			})
		}
	} else {
		// if logging config is nil, then apply default logging setup
		// add trade logger
		session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
			logger.Info(trade.String())
		})
	}

	if viper.GetBool("debug-kline") {
		session.MarketDataStream.OnKLine(func(kline types.KLine) {
			logger.WithField("marketData", "kline").Infof("kline: %+v", kline)
		})
		session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
			logger.WithField("marketData", "kline").Infof("kline closed: %+v", kline)
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

	disableMarketDataStore := environ.environmentConfig != nil && environ.environmentConfig.DisableMarketDataStore
	disableSessionTradeBuffer := environ.environmentConfig != nil && environ.environmentConfig.DisableSessionTradeBuffer
	maxSessionTradeBufferSize := 0
	if environ.environmentConfig != nil && environ.environmentConfig.MaxSessionTradeBufferSize > 0 {
		maxSessionTradeBufferSize = environ.environmentConfig.MaxSessionTradeBufferSize
	}

	session.Trades[symbol] = &types.TradeSlice{Trades: nil}

	if !disableSessionTradeBuffer {
		session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
			if trade.Symbol != symbol {
				return
			}

			session.Trades[symbol].Append(trade)

			if maxSessionTradeBufferSize > 0 {
				session.Trades[symbol].Truncate(maxSessionTradeBufferSize)
			}
		})
	}

	// session wide position
	position := &types.Position{
		Symbol:        symbol,
		BaseCurrency:  market.BaseCurrency,
		QuoteCurrency: market.QuoteCurrency,
	}
	position.BindStream(session.UserDataStream)
	session.positions[symbol] = position

	orderStore := core.NewOrderStore(symbol)
	orderStore.AddOrderUpdate = true
	orderStore.BindStream(session.UserDataStream)
	session.orderStores[symbol] = orderStore

	marketDataStore := NewMarketDataStore(symbol)
	if !disableMarketDataStore {
		if _, ok := session.marketDataStores[symbol]; !ok {
			marketDataStore.BindStream(session.MarketDataStream)
		}
	}
	session.marketDataStores[symbol] = marketDataStore

	if _, ok := session.standardIndicatorSets[symbol]; !ok {
		standardIndicatorSet := NewStandardIndicatorSet(symbol, session.MarketDataStream, marketDataStore)
		session.standardIndicatorSets[symbol] = standardIndicatorSet
	}

	// used kline intervals by the given symbol
	var klineSubscriptions = map[types.Interval]struct{}{}

	minInterval := types.Interval1m

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

			if minInterval.Seconds() > sub.Options.Interval.Seconds() {
				minInterval = sub.Options.Interval
			}

			if sub.Symbol == symbol {
				klineSubscriptions[sub.Options.Interval] = struct{}{}
			}
		}
	}

	if !(environ.environmentConfig != nil && environ.environmentConfig.DisableDefaultKLineSubscription) {
		// subscribe the 1m kline by default so we can make sure the connection persists.
		klineSubscriptions[minInterval] = struct{}{}
	}

	if !(environ.environmentConfig != nil && environ.environmentConfig.DisableHistoryKLinePreload) {
		for interval := range klineSubscriptions {
			// avoid querying the last unclosed kline
			endTime := environ.startTime
			var i int64
			for i = 0; i < KLinePreloadLimit; i += 1000 {
				var duration time.Duration = time.Duration(-i * int64(interval.Duration()))
				e := endTime.Add(duration)

				kLines, err := session.Exchange.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
					EndTime: &e,
					Limit:   1000, // indicators need at least 100
				})
				if err != nil {
					return err
				}

				if len(kLines) == 0 {
					log.Warnf("no kline data for %s %s (end time <= %s)", symbol, interval, e)
					continue
				}

				// update last prices by the given kline
				lastKLine := kLines[len(kLines)-1]
				if interval == minInterval {
					session.lastPrices[symbol] = lastKLine.Close
				}

				for _, k := range kLines {
					// let market data store trigger the update, so that the indicator could be updated too.
					marketDataStore.AddKLine(k)
				}
			}
		}

		log.Infof("%s last price: %v", symbol, session.lastPrices[symbol])
	}

	session.initializedSymbols[symbol] = struct{}{}
	return nil
}

// Indicators returns the IndicatorSet struct that maintains the kLines stream cache and price stream cache
// It also provides helper methods
func (session *ExchangeSession) Indicators(symbol string) *IndicatorSet {
	set, ok := session.indicators[symbol]
	if ok {
		return set
	}

	store, _ := session.MarketDataStore(symbol)
	set = NewIndicatorSet(symbol, session.MarketDataStream, store)
	session.indicators[symbol] = set
	return set
}

func (session *ExchangeSession) StandardIndicatorSet(symbol string) *StandardIndicatorSet {
	log.Warnf("StandardIndicatorSet() is deprecated in v1.49.0 and which will be removed in the next version, please use Indicators() instead")

	set, ok := session.standardIndicatorSets[symbol]
	if ok {
		return set
	}

	store, _ := session.MarketDataStore(symbol)
	set = NewStandardIndicatorSet(symbol, session.MarketDataStream, store)
	session.standardIndicatorSets[symbol] = set
	return set
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
	if ok {
		return s, true
	}

	s = NewMarketDataStore(symbol)
	s.BindStream(session.MarketDataStream)
	session.marketDataStores[symbol] = s
	return s, true
}

// KLine updates will be received in the order listend in intervals array
func (session *ExchangeSession) SerialMarketDataStore(
	ctx context.Context, symbol string, intervals []types.Interval, useAggTrade ...bool,
) (store *SerialMarketDataStore, ok bool) {
	st, ok := session.MarketDataStore(symbol)
	if !ok {
		return nil, false
	}
	minInterval := types.Interval1m
	for _, i := range intervals {
		if minInterval.Seconds() > i.Seconds() {
			minInterval = i
		}
	}
	store = NewSerialMarketDataStore(symbol, minInterval, useAggTrade...)
	klines, ok := st.KLinesOfInterval(minInterval)
	if !ok {
		log.Errorf("SerialMarketDataStore: cannot get %s history", minInterval)
		return nil, false
	}
	for _, interval := range intervals {
		store.Subscribe(interval)
	}
	for _, kline := range *klines {
		store.AddKLine(kline)
	}
	store.BindStream(ctx, session.MarketDataStream)
	return store, true
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

func (session *ExchangeSession) Markets() types.MarketMap {
	return session.markets
}

func (session *ExchangeSession) OrderStore(symbol string) (store *core.OrderStore, ok bool) {
	store, ok = session.orderStores[symbol]
	return store, ok
}

func (session *ExchangeSession) OrderStores() map[string]*core.OrderStore {
	return session.orderStores
}

// Subscribe save the subscription info, later it will be assigned to the stream
func (session *ExchangeSession) Subscribe(
	channel types.Channel, symbol string, options types.SubscribeOptions,
) *ExchangeSession {
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

	markets := session.Markets()
	var symbols []string
	for _, c := range currencies {
		possibleSymbols := findPossibleMarketSymbols(markets, c, fiat)
		symbols = append(symbols, possibleSymbols...)
	}

	if len(symbols) == 0 {
		return nil
	}

	tickers, err := session.Exchange.QueryTickers(ctx, symbols...)
	if err != nil || len(tickers) == 0 {
		return err
	}

	var lastTime time.Time
	for k, v := range tickers {
		// for {Crypto}/USDT markets
		// map things like BTCUSDT = {price}
		if market, ok := markets[k]; ok {
			if types.IsFiatCurrency(market.BaseCurrency) {
				session.lastPrices[k] = v.Last.Div(fixedpoint.One)
			} else {
				session.lastPrices[k] = v.Last
			}
		} else {
			session.lastPrices[k] = v.Last
		}

		if v.Time.After(lastTime) {
			lastTime = v.Time
		}
	}

	session.lastPriceUpdatedAt = lastTime
	return err
}

func (session *ExchangeSession) FindPossibleAssetSymbols() (symbols []string, err error) {
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

// newBasicPrivateExchange allocates a basic exchange instance with the user private credentials
func (session *ExchangeSession) newBasicPrivateExchange(exchangeName types.ExchangeName) (types.Exchange, error) {
	var err error
	var exMinimal types.ExchangeMinimal
	if session.Key != "" && session.Secret != "" {
		exMinimal, err = exchange2.New(exchangeName, session.Key, session.Secret, session.Passphrase)
	} else {
		exMinimal, err = exchange2.NewWithEnvVarPrefix(exchangeName, session.EnvVarPrefix)
	}

	if err != nil {
		return nil, err
	}

	if ex, ok := exMinimal.(types.Exchange); ok {
		return ex, nil
	}

	return nil, fmt.Errorf("exchange %T does not implement types.Exchange", exMinimal)
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
			ex, err = session.newBasicPrivateExchange(exchangeName)
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
	session.indicators = make(map[string]*IndicatorSet)
	session.orderStores = make(map[string]*core.OrderStore)
	session.OrderExecutor = &ExchangeOrderExecutor{
		// copy the notification system so that we can route
		Session: session,
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
		Footer:     templateutil.Render("update time {{ . }}", time.Now().Format(time.RFC822)),
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

func findPossibleMarketSymbols(markets types.MarketMap, c, fiat string) (symbols []string) {
	var tries []string
	// expand USD stable coin currencies
	if types.IsUSDFiatCurrency(fiat) {
		for _, usdFiat := range types.USDFiatCurrencies {
			tries = append(tries, c+usdFiat, usdFiat+c)
		}
	} else {
		tries = []string{c + fiat, fiat + c}
	}

	for _, try := range tries {
		if markets.Has(try) {
			symbols = append(symbols, try)
			break
		}
	}

	return symbols
}

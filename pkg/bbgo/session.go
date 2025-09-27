package bbgo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/cache"
	"github.com/c9s/bbgo/pkg/envvar"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/metrics"
	"github.com/c9s/bbgo/pkg/pricesolver"
	currency2 "github.com/c9s/bbgo/pkg/types/currency"
	"github.com/c9s/bbgo/pkg/util/templateutil"

	exchange2 "github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const defaultMaxSessionTradeBufferSize = 3500

var KLinePreloadLimit int64 = 1000

var ErrEmptyMarketInfo = errors.New("market info should not be empty, 0 markets loaded")

type ExchangeSessionMap map[string]*ExchangeSession

func (sessions ExchangeSessionMap) Filter(names []string) ExchangeSessionMap {
	mm := ExchangeSessionMap{}

	for _, name := range names {
		if session, ok := sessions[name]; ok {
			mm[name] = session
		}
	}

	return mm
}

func (sessions ExchangeSessionMap) Names() (slice []string) {
	for n := range sessions {
		slice = append(slice, n)
	}

	return slice
}

// CollectMarkets collects all markets from the sessions
//
// markets.Merge override the previous defined markets
// so we need to merge the markets in reverse order
func (sessions ExchangeSessionMap) CollectMarkets(preferredSessions []string) types.MarketMap {
	if len(preferredSessions) == 0 {
		preferredSessions = sessions.Names()
	}

	allMarkets := types.MarketMap{}

	sort.Sort(sort.Reverse(sort.StringSlice(preferredSessions)))
	for _, sessionName := range preferredSessions {
		if session, ok := sessions[sessionName]; ok {
			for symbol, market := range session.Markets() {
				allMarkets[symbol] = market
			}
		}
	}

	return allMarkets
}

func (sessions ExchangeSessionMap) AggregateBalances(
	ctx context.Context, skipUpdate bool,
) (types.BalanceMap, map[string]types.BalanceMap, error) {
	totalBalances := make(types.BalanceMap)
	sessionBalances := make(map[string]types.BalanceMap)

	var err error

	// iterate the sessions and record them
	for sessionName, session := range sessions {
		// update the account balances and the margin information
		account := session.GetAccount()
		if !skipUpdate {
			account, err = session.UpdateAccount(ctx)
			if err != nil {
				return nil, nil, err
			}
		}

		balances := account.Balances()

		sessionBalances[sessionName] = balances
		totalBalances = totalBalances.Add(balances)
	}

	return totalBalances, sessionBalances, nil
}

type ExchangeSessionConfig struct {
	Name         string             `json:"name,omitempty" yaml:"name,omitempty"`
	ExchangeName types.ExchangeName `json:"exchange" yaml:"exchange"`
	EnvVarPrefix string             `json:"envVarPrefix" yaml:"envVarPrefix"`
	Key          string             `json:"key,omitempty" yaml:"key,omitempty"`
	Secret       string             `json:"secret,omitempty" yaml:"secret,omitempty"`
	Passphrase   string             `json:"passphrase,omitempty" yaml:"passphrase,omitempty"`
	SubAccount   string             `json:"subAccount,omitempty" yaml:"subAccount,omitempty"`

	// Margin Assets Configs
	MarginInfoUpdaterInterval types.Duration `json:"marginInfoUpdaterInterval" yaml:"marginInfoUpdaterInterval"`

	MakerFeeRate fixedpoint.Value `json:"makerFeeRate" yaml:"makerFeeRate"`
	TakerFeeRate fixedpoint.Value `json:"takerFeeRate" yaml:"takerFeeRate"`

	MaxLeverage fixedpoint.Value `json:"maxLeverage,omitempty" yaml:"maxLeverage,omitempty"`

	// PublicOnly is used for setting the session to public only (without authentication, no private user data)
	PublicOnly bool `json:"publicOnly,omitempty" yaml:"publicOnly"`

	// PrivateChannels is used for filtering the private user data channel, .e.g, orders, trades, balances.. etc
	// This option is exchange-specific, currently only MAX exchange reads this option
	PrivateChannels []string `json:"privateChannels,omitempty" yaml:"privateChannels,omitempty"`

	// PrivateChannelSymbols is used for filtering the private user data channel, .e.g, order symbol subscription.
	// This option is exchange-specific, currently only Bitget exchange reads this option
	PrivateChannelSymbols []string `json:"privateChannelSymbols,omitempty" yaml:"privateChannelSymbols,omitempty"`

	Margin               bool   `json:"margin,omitempty" yaml:"margin"`
	IsolatedMargin       bool   `json:"isolatedMargin,omitempty" yaml:"isolatedMargin,omitempty"`
	IsolatedMarginSymbol string `json:"isolatedMarginSymbol,omitempty" yaml:"isolatedMarginSymbol,omitempty"`

	Futures               bool   `json:"futures,omitempty" yaml:"futures"`
	IsolatedFutures       bool   `json:"isolatedFutures,omitempty" yaml:"isolatedFutures,omitempty"`
	IsolatedFuturesSymbol string `json:"isolatedFuturesSymbol,omitempty" yaml:"isolatedFuturesSymbol,omitempty"`

	// Leverage is used for controlling the max leverage only if the exchange supports it
	SymbolLeverage map[string]int `json:"symbolLeverage,omitempty" yaml:"symbolLeverage,omitempty"`

	// AccountName is used for labeling the account name of the session
	AccountName string `json:"accountName,omitempty" yaml:"accountName,omitempty"`

	// AccountOwner is used for labeling the account owner of the session
	AccountOwner string `json:"accountOwner,omitempty" yaml:"accountOwner,omitempty"`

	// Withdrawal is used for enabling withdrawal functions
	Withdrawal bool `json:"withdrawal,omitempty" yaml:"withdrawal,omitempty"`

	UseHeikinAshi bool `json:"heikinAshi,omitempty" yaml:"heikinAshi,omitempty"`
}

// ExchangeSession presents the exchange connection Session
// It also maintains and collects the data returned from the stream.
//
//go:generate callbackgen -type ExchangeSession
type ExchangeSession struct {
	ExchangeSessionConfig `yaml:",inline"`

	// ---------------------------
	// Runtime fields
	// ---------------------------

	// The exchange account states
	Account      *types.Account `json:"-" yaml:"-"`
	accountMutex sync.Mutex

	IsInitialized bool `json:"-" yaml:"-"`

	// OrderExecutor is the default order executor for the session
	//
	// Deprecated: use GeneralOrderExecutor instead
	OrderExecutor *ExchangeOrderExecutor `json:"orderExecutor,omitempty" yaml:"orderExecutor,omitempty"`

	// UserDataStream is the user data connection stream of the exchange
	// This stream is used for managing user data, such as orders, trades, balances, etc.
	UserDataStream types.Stream `json:"-" yaml:"-"`

	// MarketDataStream is the market data connection stream of the exchange
	// This stream is used for managing market data, such as klines, trades, order books, etc.
	MarketDataStream types.Stream `json:"-" yaml:"-"`

	// UserDataConnectivity is the connectivity of the user data stream
	UserDataConnectivity *types.Connectivity `json:"-" yaml:"-"`

	// MarketDataConnectivity is the connectivity of the market data stream
	MarketDataConnectivity *types.Connectivity `json:"-" yaml:"-"`

	// Connectivity is the group of connectivity of the session
	// This is used for managing both user data and market data connectivity
	Connectivity *types.ConnectivityGroup `json:"-" yaml:"-"`

	// Subscriptions is the subscription list of the session
	// This is a read-only field when running strategy
	Subscriptions map[types.Subscription]types.Subscription `json:"-" yaml:"-"`

	// Exchange is the exchange instance, it is used for querying the exchange data or submitting orders
	Exchange types.Exchange `json:"-" yaml:"-"`

	marginInfoUpdater *MarginInfoUpdater

	// Trades collects the executed trades from the exchange
	// map: symbol -> []trade
	//
	// Trades field here is used for collecting trades in the back-test mode
	// in the production environment, we usually use trade store in the strategy instance to collect trades
	Trades map[string]*types.TradeSlice `json:"-" yaml:"-"`

	// markets defines market configuration of a symbol
	markets     map[string]types.Market
	marketMutex sync.Mutex

	// startPrices is used for backtest
	startPrices map[string]fixedpoint.Value

	lastPrices         map[string]fixedpoint.Value
	lastPriceUpdatedAt time.Time
	lastPricesMutex    sync.Mutex

	// marketDataStores contains the market data store of each market
	marketDataStores map[string]*types.MarketDataStore

	positions map[string]*types.Position

	// standard indicators of each market
	standardIndicatorSets map[string]*StandardIndicatorSet

	// indicators is the v2 api indicators
	indicators map[string]*IndicatorSet

	usedSymbols        map[string]struct{}
	initializedSymbols map[string]struct{}

	logger log.FieldLogger

	priceSolver *pricesolver.SimplePriceSolver

	AccountValueCalculator *AccountValueCalculator `json:"-" yaml:"-"`
}

// NewExchangeSession creates a new exchange session instance
// NOTE: make sure it intialize the session as the way as InitExchange
// TODO: unify the session creation and initialization (ex: calling InitExchange in NewExchangeSession)
func NewExchangeSession(name string, exchange types.Exchange) *ExchangeSession {
	userDataStream := exchange.NewStream()

	marketDataStream := exchange.NewStream()
	marketDataStream.SetPublicOnly()

	userDataConnectivity := types.NewConnectivity()
	userDataConnectivity.Bind(userDataStream)

	marketDataConnectivity := types.NewConnectivity()
	marketDataConnectivity.Bind(marketDataStream)

	connectivityGroup := types.NewConnectivityGroup(marketDataConnectivity, userDataConnectivity)
	priceSolver := pricesolver.NewSimplePriceResolver(nil)

	session := &ExchangeSession{
		ExchangeSessionConfig: ExchangeSessionConfig{
			Name:         name,
			ExchangeName: exchange.Name(),
		},

		Exchange: exchange,

		UserDataStream:       userDataStream,
		UserDataConnectivity: userDataConnectivity,

		MarketDataStream:       marketDataStream,
		MarketDataConnectivity: marketDataConnectivity,

		Connectivity: connectivityGroup,

		Subscriptions: make(map[types.Subscription]types.Subscription),
		Account:       &types.Account{},
		Trades:        make(map[string]*types.TradeSlice),

		markets:               make(map[string]types.Market, 100),
		startPrices:           make(map[string]fixedpoint.Value, 30),
		lastPrices:            make(map[string]fixedpoint.Value, 30),
		positions:             make(map[string]*types.Position),
		marketDataStores:      make(map[string]*types.MarketDataStore),
		standardIndicatorSets: make(map[string]*StandardIndicatorSet),
		indicators:            make(map[string]*IndicatorSet),
		usedSymbols:           make(map[string]struct{}),
		initializedSymbols:    make(map[string]struct{}),
		logger:                log.WithField("session", name),
		priceSolver:           priceSolver,
	}

	session.AccountValueCalculator = NewAccountValueCalculator(
		session, priceSolver, currency2.USDT,
	)

	session.OrderExecutor = &ExchangeOrderExecutor{
		// copy the notification system so that we can route
		Session: session,
	}

	return session
}

func (session *ExchangeSession) GetPriceSolver() *pricesolver.SimplePriceSolver {
	if session.priceSolver == nil {
		session.priceSolver = pricesolver.NewSimplePriceResolver(session.Markets())
	}

	return session.priceSolver
}

func (session *ExchangeSession) GetAccountValueCalculator() *AccountValueCalculator {
	return session.AccountValueCalculator
}

func (session *ExchangeSession) UnmarshalJSON(data []byte) error {
	// unmarshal the config first
	if err := json.Unmarshal(data, &session.ExchangeSessionConfig); err != nil {
		return fmt.Errorf("unmarshal exchange session config: %w", err)
	}

	// then unmarshal the rest of the fields
	if err := json.Unmarshal(data, session); err != nil {
		return fmt.Errorf("unmarshal exchange session: %w", err)
	}

	return nil
}

func (session *ExchangeSession) GetAccountLabel() string {
	var label string

	if len(session.AccountOwner) > 0 {
		label = session.AccountOwner
		if len(session.AccountName) > 0 {
			label = " (" + session.AccountName + ")"
		}
	} else if len(session.AccountName) > 0 {
		label = session.AccountName
	}

	if len(label) == 0 {
		label = os.Getenv("POD_NAME")
	}

	if len(label) == 0 {
		label = os.Getenv("HOSTNAME")
	}

	return label
}

func (session *ExchangeSession) GetAccount() (a *types.Account) {
	session.accountMutex.Lock()
	a = session.Account
	session.accountMutex.Unlock()
	return a
}

func (session *ExchangeSession) GetIsolatedSymbol() string {
	isolatedSymbol := ""
	if session.IsolatedMarginSymbol != "" {
		isolatedSymbol = session.IsolatedMarginSymbol
	} else if session.IsolatedFuturesSymbol != "" {
		isolatedSymbol = session.IsolatedFuturesSymbol
	}

	return isolatedSymbol
}

// UpdateAccount locks the account mutex and update the account object
func (session *ExchangeSession) UpdateAccount(ctx context.Context) (*types.Account, error) {
	account, err := session.Exchange.QueryAccount(ctx)
	if err != nil {
		return nil, err
	}

	isolatedSymbol := session.GetIsolatedSymbol()

	if account.MarginLevel.Sign() > 0 {
		metrics.AccountMarginLevelMetrics.With(prometheus.Labels{
			"exchange":        session.ExchangeName.String(),
			"account_type":    string(account.AccountType),
			"isolated_symbol": isolatedSymbol,
		}).Set(account.MarginLevel.Float64())
	}

	if account.MarginRatio.Sign() > 0 {
		metrics.AccountMarginRatioMetrics.With(prometheus.Labels{
			"exchange":        session.ExchangeName.String(),
			"account_type":    string(account.AccountType),
			"isolated_symbol": isolatedSymbol,
		}).Set(account.MarginRatio.Float64())
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
	if envvar.SetBool("DISABLE_MARKETS_CACHE", &disableMarketsCache); disableMarketsCache {
		markets, err = session.Exchange.QueryMarkets(ctx)
		if err != nil {
			return err
		}
	} else {
		markets, err = cache.LoadExchangeMarketsWithCache(ctx, session.Exchange)
		if err != nil {
			return err
		}
	}

	if len(markets) == 0 {
		return ErrEmptyMarketInfo
	}

	logger.Infof("%d markets loaded", len(markets))
	session.SetMarkets(markets)

	// re-allocate the priceSolver
	session.priceSolver = pricesolver.NewSimplePriceResolver(markets)
	session.AccountValueCalculator = NewAccountValueCalculator(session, session.priceSolver, currency2.USDT)
	if err := session.AccountValueCalculator.UpdatePrices(ctx); err != nil {
		return err
	}

	if len(session.SymbolLeverage) > 0 && session.Futures {
		if riskService, ok := session.Exchange.(types.ExchangeRiskService); ok {
			for symbol, leverage := range session.SymbolLeverage {
				if err := riskService.SetLeverage(ctx, symbol, leverage); err != nil {
					logger.WithError(err).Error("failed to set leverage")
				}
			}
		}
	}

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
		// replace the existing market data stream
		session.MarketDataStream = &types.HeikinAshiStream{
			StandardStreamEmitter: session.MarketDataStream.(types.StandardStreamEmitter),
		}
	}

	session.priceSolver.BindStream(session.UserDataStream)
	session.priceSolver.BindStream(session.MarketDataStream)

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
			session.setAccount(types.NewAccount())
		} else {
			logger.Infof("querying account balances...")
			account, err := retry.QueryAccountUntilSuccessful(ctx, session.Exchange)
			if err != nil {
				return err
			}

			session.setAccount(account)

			balances := account.Balances()
			session.metricsBalancesUpdater(balances)
			logger.Infof("session %s account balances:\n%s", session.Name, balances.NotZero().String())
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

		session.UserDataStream.OnFuturesPositionSnapshot(func(positions types.FuturesPositionMap) {
			session.accountMutex.Lock()
			session.Account.UpdateFuturesPositions(positions)
			session.accountMutex.Unlock()
		})

		session.UserDataStream.OnFuturesPositionUpdate(func(positions types.FuturesPositionMap) {
			session.accountMutex.Lock()
			session.Account.UpdateFuturesPositions(positions)
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
				logger.WithFields(trade.LogFields()).Info(trade.String())
			})
		}

		if environ.loggingConfig.FilledOrderOnly {
			session.UserDataStream.OnOrderUpdate(func(order types.Order) {
				if order.Status == types.OrderStatusFilled {
					logger.WithFields(order.LogFields()).Info(order.String())
				}
			})
		} else if environ.loggingConfig.Order {
			session.UserDataStream.OnOrderUpdate(func(order types.Order) {
				logger.WithFields(order.LogFields()).Info(order.String())
			})
		}
	} else {
		// if logging config is nil, then apply default logging setup
		// add trade logger
		session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
			logger.WithFields(trade.LogFields()).Info(trade.String())
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

			session.setLastPrice(kline.Symbol, session.MarketDataStream.(*types.HeikinAshiStream).LastOrigin[kline.Symbol][kline.Interval].Close)
		})
	} else {
		session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
			if _, ok := session.startPrices[kline.Symbol]; !ok {
				session.startPrices[kline.Symbol] = kline.Open
			}

			session.setLastPrice(kline.Symbol, kline.Close)
		})
	}

	session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {
		session.setLastPrice(trade.Symbol, trade.Price)
	})

	// session-wide max borrowable updating worker
	if session.Margin {
		marginUpdater := NewMarginInfoUpdaterFromExchange(session.Exchange)
		session.marginInfoUpdater = &marginUpdater

		if session.MarginInfoUpdaterInterval == 0 {
			session.MarginInfoUpdaterInterval = types.Duration(30 * time.Minute)
		}
		session.logger.Infof("max borrowable update interval: %s", session.MarginInfoUpdaterInterval.Duration())
		go session.marginInfoUpdater.Run(ctx, session.MarginInfoUpdaterInterval)
	}

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

	market, ok := session.Market(symbol)
	if !ok {
		return fmt.Errorf("market %s is not defined", symbol)
	}

	if environ.environmentConfig != nil {
		session.logger.Infof("environment config: %+v", environ.environmentConfig)
	}

	disableMarketDataStore := environ.environmentConfig != nil && environ.environmentConfig.DisableMarketDataStore
	disableSessionTradeBuffer := environ.environmentConfig != nil && environ.environmentConfig.DisableSessionTradeBuffer

	maxSessionTradeBufferSize := defaultMaxSessionTradeBufferSize
	if environ.environmentConfig != nil && environ.environmentConfig.MaxSessionTradeBufferSize > 0 {
		maxSessionTradeBufferSize = environ.environmentConfig.MaxSessionTradeBufferSize
	}

	session.Trades[symbol] = types.NewTradeSlice(100)

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

	marketDataStore := types.NewMarketDataStore(symbol)
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
					session.setLastPrice(symbol, lastKLine.Close)
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

// StandardIndicatorSet
// Deprecated: use Indicators(symbol) instead
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

	market, ok := session.Market(symbol)
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
func (session *ExchangeSession) MarketDataStore(symbol string) (s *types.MarketDataStore, ok bool) {
	s, ok = session.marketDataStores[symbol]
	if ok {
		return s, true
	}

	s = types.NewMarketDataStore(symbol)
	s.BindStream(session.MarketDataStream)
	session.marketDataStores[symbol] = s
	return s, true
}

// KLine updates will be received in the order listend in intervals array
func (session *ExchangeSession) SerialMarketDataStore(
	ctx context.Context, symbol string, intervals []types.Interval, useAggTrade ...bool,
) (store *types.SerialMarketDataStore, ok bool) {
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
	store = types.NewSerialMarketDataStore(symbol, minInterval, useAggTrade...)
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

func (session *ExchangeSession) StartPrice(symbol string) (price fixedpoint.Value, ok bool) {
	price, ok = session.startPrices[symbol]
	return price, ok
}

func (session *ExchangeSession) LastPrice(symbol string) (price fixedpoint.Value, ok bool) {
	session.lastPricesMutex.Lock()
	defer session.lastPricesMutex.Unlock()

	price, ok = session.lastPrices[symbol]
	return price, ok
}

func (session *ExchangeSession) AllLastPrices() map[string]fixedpoint.Value {
	return session.LastPrices()
}

func (session *ExchangeSession) LastPrices() map[string]fixedpoint.Value {
	session.lastPricesMutex.Lock()
	defer session.lastPricesMutex.Unlock()

	return session.lastPrices
}

func (session *ExchangeSession) Market(symbol string) (market types.Market, ok bool) {
	session.marketMutex.Lock()
	defer session.marketMutex.Unlock()

	market, ok = session.markets[symbol]
	return market, ok
}

func (session *ExchangeSession) Markets() types.MarketMap {
	session.marketMutex.Lock()
	defer session.marketMutex.Unlock()

	return session.markets
}

func (session *ExchangeSession) UpdateMarkets(ctx context.Context) error {
	markets, err := session.Exchange.QueryMarkets(ctx)
	if err != nil {
		return err
	}

	session.SetMarkets(markets)
	return nil
}

func (session *ExchangeSession) SetMarkets(markets types.MarketMap) {
	session.marketMutex.Lock()
	defer session.marketMutex.Unlock()

	session.markets = markets
	if session.priceSolver != nil {
		session.priceSolver.SetMarkets(markets)
	}
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
	markets := session.Markets()
	symbols := make([]string, 0, 50)
	for _, c := range currencies {
		for symbol := range markets.FindAssetMarkets(c) {
			symbols = append(symbols, symbol)
		}
	}

	if len(symbols) == 0 {
		return nil
	}

	tickers, err := session.Exchange.QueryTickers(ctx, symbols...)
	if err != nil || len(tickers) == 0 {
		return err
	}

	priceSolver := session.GetPriceSolver()

	var lastTime time.Time
	for k, v := range tickers {
		validPrice := v.GetValidPrice()
		priceSolver.Update(k, validPrice)

		// for {Crypto}/USDT markets
		// map things like BTCUSDT = {price}
		if market, ok := markets[k]; ok {
			if currency2.IsFiatCurrency(market.BaseCurrency) {
				session.setLastPrice(k, validPrice.Div(fixedpoint.One))
			} else {
				session.setLastPrice(k, validPrice)
			}
		} else {
			session.setLastPrice(k, v.Last)
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

	for _, currency := range currency2.FiatCurrencies {
		if balance, ok := balances[currency]; ok && balance.Total().Sign() > 0 {
			fiatAssets = append(fiatAssets, currency)
		}
	}

	var symbolMap = map[string]struct{}{}

	for _, market := range session.Markets() {
		// ignore the markets that are not fiat currency markets
		if !slices.Contains(fiatAssets, market.QuoteCurrency) {
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
		options := exchange2.Options{
			exchange2.OptionKeyAPIKey:        session.Key,
			exchange2.OptionKeyAPISecret:     session.Secret,
			exchange2.OptionKeyAPIPassphrase: session.Passphrase,
		}
		exMinimal, err = exchange2.New(exchangeName, options)
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

	session.UserDataConnectivity = types.NewConnectivity()
	session.UserDataConnectivity.Bind(session.UserDataStream)

	session.MarketDataConnectivity = types.NewConnectivity()
	session.MarketDataConnectivity.Bind(session.MarketDataStream)

	session.Connectivity = types.NewConnectivityGroup(session.MarketDataConnectivity, session.MarketDataConnectivity)

	// pointer fields
	session.Subscriptions = make(map[types.Subscription]types.Subscription)
	session.Account = types.NewAccount()
	session.Trades = make(map[string]*types.TradeSlice)

	session.markets = make(map[string]types.Market)
	session.lastPrices = make(map[string]fixedpoint.Value)
	session.startPrices = make(map[string]fixedpoint.Value)
	session.marketDataStores = make(map[string]*types.MarketDataStore)
	session.positions = make(map[string]*types.Position)
	session.standardIndicatorSets = make(map[string]*StandardIndicatorSet)
	session.indicators = make(map[string]*IndicatorSet)
	session.OrderExecutor = &ExchangeOrderExecutor{
		// copy the notification system so that we can route
		Session: session,
	}

	session.usedSymbols = make(map[string]struct{})
	session.initializedSymbols = make(map[string]struct{})
	session.logger = log.WithField("session", name)
	return nil
}

func (session *ExchangeSession) MarginType() types.MarginType {
	if session.Margin {
		if session.IsolatedMargin {
			return types.MarginTypeIsolatedMargin
		} else {
			return types.MarginTypeCrossMargin
		}
	}

	return types.MarginTypeSpot
}

func (session *ExchangeSession) metricsBalancesUpdater(balances types.BalanceMap) {
	labels := prometheus.Labels{
		"session":     session.Name,
		"exchange":    session.ExchangeName.String(),
		"margin_type": string(session.MarginType()),
		"symbol":      session.IsolatedMarginSymbol,
	}

	metricsTotalBalancesCurry := metricsTotalBalances.MustCurryWith(labels)
	metricsBalanceNetMetricsCurry := metricsBalanceNetMetrics.MustCurryWith(labels)
	metricsBalanceAvailableMetricsCurry := metricsBalanceAvailableMetrics.MustCurryWith(labels)
	metricsBalanceLockedMetricsCurry := metricsBalanceLockedMetrics.MustCurryWith(labels)
	metricsBalanceDebtMetricsCurry := metricsBalanceDebtMetrics.MustCurryWith(labels)
	metricsBalanceBorrowedMetricsCurry := metricsBalanceBorrowedMetrics.MustCurryWith(labels)
	metricsBalanceInterestMetricsCurry := metricsBalanceInterestMetrics.MustCurryWith(labels)

	for currency, balance := range balances {
		curLabels := prometheus.Labels{
			"currency": currency,
		}

		metricsTotalBalancesCurry.With(curLabels).Set(balance.Total().Float64())
		metricsBalanceNetMetricsCurry.With(curLabels).Set(balance.Net().Float64())
		metricsBalanceAvailableMetricsCurry.With(curLabels).Set(balance.Available.Float64())
		metricsBalanceLockedMetricsCurry.With(curLabels).Set(balance.Locked.Float64())

		// margin metrics
		metricsBalanceDebtMetricsCurry.With(curLabels).Set(balance.Debt().Float64())
		metricsBalanceBorrowedMetricsCurry.With(curLabels).Set(balance.Borrowed.Float64())
		metricsBalanceInterestMetricsCurry.With(curLabels).Set(balance.Interest.Float64())

		metricsLastUpdateTimeMetrics.With(prometheus.Labels{
			"session":     session.Name,
			"exchange":    session.ExchangeName.String(),
			"margin_type": string(session.MarginType()),
			"channel":     "user",
			"data_type":   "balance",
			"symbol":      "",
			"currency":    currency,
		}).SetToCurrentTime()
	}
}

func (session *ExchangeSession) metricsOrderUpdater(order types.Order) {
	metricsLastUpdateTimeMetrics.With(prometheus.Labels{
		"session":     session.Name,
		"exchange":    session.ExchangeName.String(),
		"margin_type": string(session.MarginType()),
		"channel":     "user",
		"data_type":   "order",
		"symbol":      order.Symbol,
		"currency":    "",
	}).SetToCurrentTime()
}

func (session *ExchangeSession) metricsTradeUpdater(trade types.Trade) {
	labels := prometheus.Labels{
		"session":     session.Name,
		"exchange":    session.ExchangeName.String(),
		"margin_type": string(session.MarginType()),
		"side":        trade.Side.String(),
		"symbol":      trade.Symbol,
		"liquidity":   trade.Liquidity(),
	}
	metricsTradingVolume.With(labels).Add(trade.Quantity.Mul(trade.Price).Float64())
	metricsTradesTotal.With(labels).Inc()
	metricsLastUpdateTimeMetrics.With(prometheus.Labels{
		"session":     session.Name,
		"exchange":    session.ExchangeName.String(),
		"margin_type": string(session.MarginType()),
		"channel":     "user",
		"data_type":   "trade",
		"symbol":      trade.Symbol,
		"currency":    "",
	}).SetToCurrentTime()
}

func (session *ExchangeSession) bindMarketDataStreamMetrics(stream types.Stream) {
	stream.OnBookUpdate(func(book types.SliceOrderBook) {
		metricsLastUpdateTimeMetrics.With(prometheus.Labels{
			"session":     session.Name,
			"exchange":    session.ExchangeName.String(),
			"margin_type": string(session.MarginType()),
			"channel":     "market",
			"data_type":   "book",
			"symbol":      book.Symbol,
			"currency":    "",
		}).SetToCurrentTime()
	})
	stream.OnKLineClosed(func(kline types.KLine) {
		metricsLastUpdateTimeMetrics.With(prometheus.Labels{
			"session":     session.Name,
			"exchange":    session.ExchangeName.String(),
			"margin_type": string(session.MarginType()),
			"channel":     "market",
			"data_type":   "kline",
			"symbol":      kline.Symbol,
			"currency":    "",
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
			"channel":     "user",
			"session":     session.Name,
			"exchange":    session.ExchangeName.String(),
			"margin_type": string(session.MarginType()),
			"symbol":      session.IsolatedMarginSymbol,
		}).Set(0.0)
	})
	stream.OnConnect(func() {
		metricsConnectionStatus.With(prometheus.Labels{
			"channel":     "user",
			"session":     session.Name,
			"exchange":    session.ExchangeName.String(),
			"margin_type": string(session.MarginType()),
			"symbol":      session.IsolatedMarginSymbol,
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

// Expose margin updator APIs via ExchangeSession

func (session *ExchangeSession) AddMarginAssets(
	assets ...string,
) {
	if session.marginInfoUpdater == nil {
		return
	}
	session.logger.Infof("adding margin assets: %v", assets)
	session.marginInfoUpdater.AddAssets(assets...)
}

func (session *ExchangeSession) OnMaxBorrowable(cb MaxBorrowableCallback) {
	if session.marginInfoUpdater == nil {
		return
	}
	session.marginInfoUpdater.OnMaxBorrowable(cb)
}

func (session *ExchangeSession) UpdateMaxBorrowable(ctx context.Context) {
	if session.marginInfoUpdater == nil {
		return
	}
	session.marginInfoUpdater.UpdateMaxBorrowable(ctx)
}

func (session *ExchangeSession) setLastPrice(symbol string, price fixedpoint.Value) {
	session.lastPricesMutex.Lock()
	defer session.lastPricesMutex.Unlock()

	session.lastPrices[symbol] = price
}

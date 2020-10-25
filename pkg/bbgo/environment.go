package bbgo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/store"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var LoadedExchangeStrategies = make(map[string]SingleExchangeStrategy)

func RegisterExchangeStrategy(key string, configmap SingleExchangeStrategy) {
	LoadedExchangeStrategies[key] = configmap
}

var LoadedCrossExchangeStrategies = make(map[string]CrossExchangeStrategy)

func RegisterCrossExchangeStrategy(key string, configmap CrossExchangeStrategy) {
	LoadedCrossExchangeStrategies[key] = configmap
}

type TradeReporter struct {
	notifier Notifier

	channel       string
	channelRoutes map[*regexp.Regexp]string
}

func NewTradeReporter(notifier Notifier) *TradeReporter {
	return &TradeReporter{
		notifier:      notifier,
		channelRoutes: make(map[*regexp.Regexp]string),
	}
}

func (reporter *TradeReporter) Channel(channel string) *TradeReporter {
	reporter.channel = channel
	return reporter
}

func (reporter *TradeReporter) ChannelBySymbol(routes map[string]string) *TradeReporter {
	for pattern, channel := range routes {
		reporter.channelRoutes[regexp.MustCompile(pattern)] = channel
	}

	return reporter
}

func (reporter *TradeReporter) getChannel(symbol string) string {
	for pattern, channel := range reporter.channelRoutes {
		if pattern.MatchString(symbol) {
			return channel
		}
	}

	return reporter.channel
}

func (reporter *TradeReporter) Report(trade types.Trade) {
	var channel = reporter.getChannel(trade.Symbol)

	var text = util.Render(`:handshake: {{ .Symbol }} {{ .Side }} Trade Execution @ {{ .Price  }}`, trade)
	if err := reporter.notifier.NotifyTo(channel, text, trade); err != nil {
		log.WithError(err).Errorf("notifier error, channel=%s", channel)
	}
}

// Environment presents the real exchange data layer
type Environment struct {
	TradeService *service.TradeService
	TradeSync    *service.TradeSync

	tradeScanTime time.Time
	sessions      map[string]*ExchangeSession

	tradeReporter *TradeReporter
}

// NewDefaultEnvironment prepares the exchange sessions from the viper settings.
func NewDefaultEnvironment(db *sqlx.DB) *Environment {
	environment := NewEnvironment(db)

	for _, n := range SupportedExchanges {
		if viper.IsSet(string(n) + "-api-key") {
			exchange, err := cmdutil.NewExchange(n)
			if err != nil {
				panic(err)
			}

			environment.AddExchange(string(n), exchange)
		}
	}

	return environment
}

func NewEnvironment(db *sqlx.DB) *Environment {
	tradeService := &service.TradeService{DB: db}
	return &Environment{
		TradeService: tradeService,
		TradeSync: &service.TradeSync{
			Service: tradeService,
		},
		tradeScanTime: time.Now().AddDate(0, 0, -7), // sync from 7 days ago
		sessions:      make(map[string]*ExchangeSession),
	}
}

func (environ *Environment) AddExchange(name string, exchange types.Exchange) (session *ExchangeSession) {
	session = NewExchangeSession(name, exchange)
	environ.sessions[name] = session
	return session
}

func (environ *Environment) ReportTrade(notifier Notifier) *TradeReporter {
	environ.tradeReporter = NewTradeReporter(notifier)
	return environ.tradeReporter
}

func (environ *Environment) Init(ctx context.Context) (err error) {
	for _, session := range environ.sessions {
		var markets types.MarketMap

		err = WithCache(fmt.Sprintf("%s-markets", session.Exchange.Name()), &markets, func() (interface{}, error) {
			return session.Exchange.QueryMarkets(ctx)
		})
		if err != nil {
			return err
		}

		if len(markets) == 0 {
			return errors.Errorf("market config should not be empty")
		}

		session.markets = markets

		if session.tradeReporter != nil {
			session.Stream.OnTrade(func(trade types.Trade) {
				session.tradeReporter.Report(trade)
			})
		} else if environ.tradeReporter != nil {
			session.Stream.OnTrade(func(trade types.Trade) {
				environ.tradeReporter.Report(trade)
			})
		}
	}

	return nil
}

// SetTradeScanTime overrides the default trade scan time (-7 days)
func (environ *Environment) SetTradeScanTime(t time.Time) *Environment {
	environ.tradeScanTime = t
	return environ
}

func (environ *Environment) Connect(ctx context.Context) error {
	var err error

	for n := range environ.sessions {
		// avoid using the placeholder variable for the session because we use that in the callbacks
		var session = environ.sessions[n]
		var log = log.WithField("session", n)

		loadedSymbols := make(map[string]struct{})
		for _, s := range session.Subscriptions {
			symbol := strings.ToUpper(s.Symbol)
			loadedSymbols[symbol] = struct{}{}

			log.Infof("subscribing %s %s %v", s.Symbol, s.Channel, s.Options)
			session.Stream.Subscribe(s.Channel, s.Symbol, s.Options)
		}

		// trade sync and market data store depends on subscribed symbols so we have to do this here.
		for symbol := range loadedSymbols {
			log.Infof("syncing trades from %s for symbol %s...", session.Exchange.Name(), symbol)
			if err := environ.TradeSync.Sync(ctx, session.Exchange, symbol, environ.tradeScanTime); err != nil {
				return err
			}

			var trades []types.Trade

			tradingFeeCurrency := session.Exchange.PlatformFeeCurrency()
			if strings.HasPrefix(symbol, tradingFeeCurrency) {
				trades, err = environ.TradeService.QueryForTradingFeeCurrency(symbol, tradingFeeCurrency)
			} else {
				trades, err = environ.TradeService.Query(symbol)
			}

			if err != nil {
				return err
			}

			log.Infof("symbol %s: %d trades loaded", symbol, len(trades))
			session.Trades[symbol] = trades

			currentPrice, err := session.Exchange.QueryAveragePrice(ctx, symbol)
			if err != nil {
				return err
			}

			session.lastPrices[symbol] = currentPrice

			marketDataStore := store.NewMarketDataStore(symbol)
			marketDataStore.BindStream(session.Stream)

			session.marketDataStores[symbol] = marketDataStore
		}

		log.Infof("querying balances...")
		balances, err := session.Exchange.QueryAccountBalances(ctx)
		if err != nil {
			return err
		}

		session.Account.UpdateBalances(balances)
		session.Account.BindStream(session.Stream)

		// update last prices
		session.Stream.OnKLineClosed(func(kline types.KLine) {
			log.Infof("kline closed: %+v", kline)
			session.lastPrices[kline.Symbol] = kline.Close
			session.marketDataStores[kline.Symbol].AddKLine(kline)
		})

		session.Stream.OnTrade(func(trade types.Trade) {
			// append trades
			session.Trades[trade.Symbol] = append(session.Trades[trade.Symbol], trade)

			if err := environ.TradeService.Insert(trade); err != nil {
				log.WithError(err).Errorf("trade insert error: %+v", trade)
			}
		})

		if len(session.Subscriptions) == 0 {
			log.Warnf("no subscriptions, exchange session %s will not be connected", session.Name)
			continue
		}

		log.Infof("connecting session %s...", session.Name)
		if err := session.Stream.Connect(ctx); err != nil {
			return err
		}
	}

	return nil
}

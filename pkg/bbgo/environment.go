package bbgo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/store"
	"github.com/c9s/bbgo/pkg/types"
)

var LoadedExchangeStrategies = make(map[string]SingleExchangeStrategy)

func RegisterExchangeStrategy(key string, configmap SingleExchangeStrategy) {
	LoadedExchangeStrategies[key] = configmap
}

var LoadedCrossExchangeStrategies = make(map[string]CrossExchangeStrategy)

func RegisterCrossExchangeStrategy(key string, configmap CrossExchangeStrategy) {
	LoadedCrossExchangeStrategies[key] = configmap
}

// Environment presents the real exchange data layer
type Environment struct {
	TradeService *service.TradeService
	TradeSync    *service.TradeSync

	tradeScanTime time.Time
	sessions      map[string]*ExchangeSession
}

func NewEnvironment() *Environment {
	return &Environment{
		// default trade scan time
		tradeScanTime: time.Now().AddDate(0, 0, -7), // sync from 7 days ago
		sessions:      make(map[string]*ExchangeSession),
	}
}

func (environ *Environment) SyncTrades(db *sqlx.DB) *Environment {
	environ.TradeService = &service.TradeService{DB: db}
	environ.TradeSync = &service.TradeSync{
		Service: environ.TradeService,
	}

	return environ
}

func (environ *Environment) AddExchange(name string, exchange types.Exchange) (session *ExchangeSession) {
	session = NewExchangeSession(name, exchange)
	environ.sessions[name] = session
	return session
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
	}

	return nil
}

// SyncTradesFrom overrides the default trade scan time (-7 days)
func (environ *Environment) SyncTradesFrom(t time.Time) *Environment {
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
			var trades []types.Trade

			if environ.TradeSync != nil {
				log.Infof("syncing trades from %s for symbol %s...", session.Exchange.Name(), symbol)
				if err := environ.TradeSync.Sync(ctx, session.Exchange, symbol, environ.tradeScanTime); err != nil {
					return err
				}

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
			}

			session.Trades[symbol] = trades

			currentPrice, err := session.Exchange.QueryAveragePrice(ctx, symbol)
			if err != nil {
				return err
			}

			session.lastPrices[symbol] = currentPrice

			marketDataStore := store.NewMarketDataStore(symbol)
			marketDataStore.BindStream(session.Stream)

			standardIndicatorSet := NewStandardIndicatorSet(symbol)
			standardIndicatorSet.BindMarketDataStore(marketDataStore)

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

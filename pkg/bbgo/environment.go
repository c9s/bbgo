package bbgo

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

// Environment presents the real exchange data layer
type Environment struct {
	TradeService *service.TradeService
	TradeSync    *service.TradeSync

	sessions map[string]*ExchangeSession
}

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
		sessions: make(map[string]*ExchangeSession),
	}
}

func (environ *Environment) AddExchange(name string, exchange types.Exchange) (session *ExchangeSession) {
	session = &ExchangeSession{
		Name:          name,
		Exchange:      exchange,
		Subscriptions: make(map[types.Subscription]types.Subscription),
		Markets:       make(map[string]types.Market),
		Trades:        make(map[string][]types.Trade),
		LastPrices:    make(map[string]float64),
	}

	environ.sessions[name] = session
	return session
}

func (environ *Environment) Init(ctx context.Context) (err error) {
	startTime := time.Now().AddDate(0, 0, -7) // sync from 7 days ago

	for _, session := range environ.sessions {
		loadedSymbols := make(map[string]struct{})
		for _, sub := range session.Subscriptions {
			loadedSymbols[sub.Symbol] = struct{}{}
		}

		markets, err := session.Exchange.QueryMarkets(ctx)
		if err != nil {
			return err
		}

		session.Markets = markets

		for symbol := range loadedSymbols {
			if err := environ.TradeSync.Sync(ctx, session.Exchange, symbol, startTime); err != nil {
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

			logrus.Infof("symbol %s: %d trades loaded", symbol, len(trades))
			session.Trades[symbol] = trades

			currentPrice, err := session.Exchange.QueryAveragePrice(ctx, symbol)
			if err != nil {
				return err
			}

			session.LastPrices[symbol] = currentPrice
		}

		balances, err := session.Exchange.QueryAccountBalances(ctx)
		if err != nil {
			return err
		}

		stream := session.Exchange.NewStream()

		session.Stream = stream

		session.Account = &Account{balances: balances}
		session.Account.BindStream(session.Stream)

		marketDataStore := NewMarketDataStore()
		marketDataStore.BindStream(session.Stream)

		// update last prices
		session.Stream.OnKLineClosed(func(kline types.KLine) {
			session.LastPrices[kline.Symbol] = kline.Close
		})

		session.Stream.OnTrade(func(trade *types.Trade) {
			// append trades
			session.Trades[trade.Symbol] = append(session.Trades[trade.Symbol], *trade)

			if err := environ.TradeService.Insert(*trade); err != nil {
				logrus.WithError(err).Errorf("trade insert error: %+v", *trade)
			}
		})
	}

	return nil
}

func (environ *Environment) Connect(ctx context.Context) error {
	for _, session := range environ.sessions {
		for _, s := range session.Subscriptions {
			logrus.Infof("subscribing %s %s %v", s.Symbol, s.Channel, s.Options)
			session.Stream.Subscribe(s.Channel, s.Symbol, s.Options)
		}

		if err := session.Stream.Connect(ctx); err != nil {
			return err
		}
	}

	return nil
}

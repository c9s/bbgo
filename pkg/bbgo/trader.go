package bbgo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/accounting"
	"github.com/c9s/bbgo/pkg/bbgo/config"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"

	_ "github.com/go-sql-driver/mysql"
)

// MarketStrategy represents the single Exchange strategy
type MarketStrategy interface {
	OnLoad(tradingContext *Context, trader types.Trader) error
	OnNewStream(stream types.Stream) error
}

type ExchangeSession struct {
	// Session name
	Name string

	// The exchange account states
	Account *Account

	// Stream is the connection stream of the exchange
	Stream types.Stream

	Subscriptions []types.Subscription

	Exchange types.Exchange

	loadedSymbols map[string]struct{}

	// Markets defines market configuration of a symbol
	Markets map[string]types.Market

	// Trades collects the executed trades from the exchange
	// map: symbol -> []trade
	Trades map[string][]types.Trade
}

func (session *ExchangeSession) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) *ExchangeSession {
	session.Subscriptions = append(session.Subscriptions, types.Subscription{
		Channel: channel,
		Symbol:  symbol,
		Options: options,
	})

	return session
}

// Environment presents the real exchange data layer
type Environment struct {
	TradeService *service.TradeService
	TradeSync    *service.TradeSync

	ExchangeSessions map[string]*ExchangeSession
}

func NewEnvironment(db *sqlx.DB) *Environment {
	tradeService := &service.TradeService{DB: db}
	return &Environment{
		TradeService: tradeService,
		TradeSync: &service.TradeSync{
			Service: tradeService,
		},
		ExchangeSessions: make(map[string]*ExchangeSession),
	}
}

func (environ *Environment) AddExchange(name string, exchange types.Exchange) (session *ExchangeSession) {
	session = &ExchangeSession{
		Name:     name,
		Exchange: exchange,
		Markets:  make(map[string]types.Market),
		Trades:   make(map[string][]types.Trade),
	}

	environ.ExchangeSessions[name] = session
	return session
}

func (environ *Environment) Connect(ctx context.Context) (err error) {
	startTime := time.Now().AddDate(0, 0, -7) // sync from 7 days ago

	for _, session := range environ.ExchangeSessions {
		loadedSymbols := make(map[string]struct{})
		for _, sub := range session.Subscriptions {
			loadedSymbols[sub.Symbol] = struct{}{}
		}

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

			log.Infof("symbol %s: %d trades loaded", symbol, len(trades))
			session.Trades[symbol] = trades
		}

		session.Account, err = LoadAccount(ctx, session.Exchange)
		if err != nil {
			return err
		}

		session.Stream = session.Exchange.NewStream()
		session.Stream.OnTrade(func(trade *types.Trade) {
			// append trades
			session.Trades[trade.Symbol] = append(session.Trades[trade.Symbol], *trade)

			if err := environ.TradeService.Insert(*trade); err != nil {
				log.WithError(err).Errorf("trade insert error: %+v", *trade)
			}
		})

		if err := session.Stream.Connect(ctx); err != nil {
			return err
		}
	}

	return err
}

type Trader struct {
	Symbol       string
	TradeService *service.TradeService
	TradeSync    *service.TradeSync

	// Context is trading Context
	Context *Context

	Exchange types.Exchange

	reportTimer *time.Timer

	ProfitAndLossCalculator *accounting.ProfitAndLossCalculator

	Account *Account

	Notifiers []Notifier

	ExchangeSessions map[string]*ExchangeSession
}

func NewTrader(db *sqlx.DB, exchange types.Exchange, symbol string) *Trader {
	tradeService := &service.TradeService{DB: db}
	return &Trader{
		Symbol:       symbol,
		Exchange:     exchange,
		TradeService: tradeService,
		TradeSync: &service.TradeSync{
			Service: tradeService,
		},
	}
}

func (trader *Trader) AddNotifier(notifier Notifier) {
	trader.Notifiers = append(trader.Notifiers, notifier)
}

func (trader *Trader) Connect(ctx context.Context) (err error) {
	log.Info("syncing trades from exchange...")
	startTime := time.Now().AddDate(0, 0, -7) // sync from 7 days ago

	for _, session := range trader.ExchangeSessions {
		for symbol := range session.loadedSymbols {
			market, ok := types.FindMarket(symbol)
			if !ok {
				return errors.Errorf("market %s is not defined", symbol)
			}

			session.Markets[symbol] = market


			if err := trader.TradeSync.Sync(ctx, session.Exchange, symbol, startTime); err != nil {
				return err
			}

			var trades []types.Trade

			tradingFeeCurrency := session.Exchange.PlatformFeeCurrency()
			if strings.HasPrefix(symbol, tradingFeeCurrency) {
				trades, err = trader.TradeService.QueryForTradingFeeCurrency(symbol, tradingFeeCurrency)
			} else {
				trades, err = trader.TradeService.Query(symbol)
			}

			if err != nil {
				return err
			}

			log.Infof("symbol %s: %d trades loaded", symbol, len(trades))
			session.Trades[symbol] = trades

			stockManager := &StockDistribution{
				Symbol:             symbol,
				TradingFeeCurrency: tradingFeeCurrency,
			}

			checkpoints, err := stockManager.AddTrades(trades)
			if err != nil {
				return err
			}

			log.Infof("symbol %s: found stock checkpoints: %+v", symbol, checkpoints)
		}

		session.Account, err = LoadAccount(ctx, session.Exchange)
		if err != nil {
			return err
		}

		session.Stream = session.Exchange.NewStream()
		if err != nil {
			return err
		}

		if err := session.Stream.Connect(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (trader *Trader) Initialize(ctx context.Context, startTime time.Time) error {
	// query all trades from database so that we can get the correct pnl
	var err error
	var trades []types.Trade
	tradingFeeCurrency := trader.Exchange.PlatformFeeCurrency()
	if strings.HasPrefix(trader.Symbol, tradingFeeCurrency) {
		trades, err = trader.TradeService.QueryForTradingFeeCurrency(trader.Symbol, tradingFeeCurrency)
	} else {
		trades, err = trader.TradeService.Query(trader.Symbol)
	}

	if err != nil {
		return err
	}

	log.Infof("%d trades loaded", len(trades))

	stockManager := &StockDistribution{
		Symbol:             trader.Symbol,
		TradingFeeCurrency: tradingFeeCurrency,
	}

	checkpoints, err := stockManager.AddTrades(trades)
	if err != nil {
		return err
	}

	log.Infof("found checkpoints: %+v", checkpoints)

	market, ok := types.FindMarket(trader.Symbol)
	if !ok {
		return fmt.Errorf("%s market not found", trader.Symbol)
	}

	currentPrice, err := trader.Exchange.QueryAveragePrice(ctx, trader.Symbol)
	if err != nil {
		return err
	}

	trader.Context = &Context{
		CurrentPrice: currentPrice,
		Symbol:       trader.Symbol,
		Market:       market,
		StockManager: stockManager,
	}

	/*
		if len(checkpoints) > 0 {
			// get the last checkpoint
			idx := checkpoints[len(checkpoints)-1]
			if idx < len(trades)-1 {
				trades = trades[idx:]
				firstTrade := trades[0]
				pnlStartTime = firstTrade.Time
				notifier.Notify("%s Found the latest trade checkpoint %s", firstTrade.Symbol, firstTrade.Time, firstTrade)
			}
		}
	*/

	trader.ProfitAndLossCalculator = &accounting.ProfitAndLossCalculator{
		TradingFeeCurrency: tradingFeeCurrency,
		Symbol:             trader.Symbol,
		StartTime:          startTime,
		CurrentPrice:       currentPrice,
		Trades:             trades,
	}

	account, err := LoadAccount(ctx, trader.Exchange)
	if err != nil {
		return err
	}

	trader.Account = account
	trader.Context.Balances = account.Balances
	account.Print()

	return nil
}

func (trader *Trader) RunStrategyWithHotReload(ctx context.Context, strategy MarketStrategy, configFile string) (chan struct{}, error) {
	var done = make(chan struct{})
	var configWatcherDone = make(chan struct{})

	log.Infof("watching config file: %v", configFile)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	defer watcher.Close()

	if err := watcher.Add(configFile); err != nil {
		return nil, err
	}

	go func() {
		strategyContext, strategyCancel := context.WithCancel(ctx)
		defer strategyCancel()
		defer close(done)

		traderDone, err := trader.RunStrategy(strategyContext, strategy)
		if err != nil {
			return
		}

		var configReloadTimer *time.Timer = nil
		defer close(configWatcherDone)

		for {
			select {

			case <-ctx.Done():
				return

			case <-traderDone:
				log.Infof("reloading config file %s", configFile)
				if err := config.LoadConfigFile(configFile, strategy); err != nil {
					log.WithError(err).Error("error load config file")
				}

				trader.Notify("config reloaded, restarting trader")

				traderDone, err = trader.RunStrategy(strategyContext, strategy)
				if err != nil {
					log.WithError(err).Error("[trader] error:", err)
					return
				}

			case event := <-watcher.Events:
				log.Infof("[fsnotify] event: %+v", event)

				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Info("[fsnotify] modified file:", event.Name)
				}

				if configReloadTimer != nil {
					configReloadTimer.Stop()
				}

				configReloadTimer = time.AfterFunc(3*time.Second, func() {
					strategyCancel()
				})

			case err := <-watcher.Errors:
				log.WithError(err).Error("[fsnotify] error:", err)
				return

			}
		}
	}()

	return done, nil
}

func (trader *Trader) RunStrategy(ctx context.Context, strategy MarketStrategy) (chan struct{}, error) {
	if err := strategy.OnLoad(trader.Context, trader); err != nil {
		return nil, err
	}

	stream := trader.Exchange.NewStream()

	// bind kline store to the stream
	klineStore := NewMarketDataStore()
	klineStore.BindPrivateStream(stream)

	trader.Account.BindPrivateStream(stream)

	if err := strategy.OnNewStream(stream); err != nil {
		return nil, err
	}

	trader.reportTimer = time.AfterFunc(1*time.Second, func() {
		trader.reportPnL()
	})

	stream.OnTrade(func(trade *types.Trade) {
		if trade.Symbol != trader.Symbol {
			return
		}

		if err := trader.TradeService.Insert(*trade); err != nil {
			log.WithError(err).Error("trade insert error")
		}

		trader.NotifyTrade(trade)
		trader.ProfitAndLossCalculator.AddTrade(*trade)
		_, err := trader.Context.StockManager.AddTrades([]types.Trade{*trade})
		if err != nil {
			log.WithError(err).Error("stock manager load trades error")
		}

		if trader.reportTimer != nil {
			trader.reportTimer.Stop()
		}

		trader.reportTimer = time.AfterFunc(1*time.Minute, func() {
			trader.reportPnL()
		})
	})

	stream.OnKLineClosed(func(kline types.KLine) {
		trader.ProfitAndLossCalculator.SetCurrentPrice(kline.Close)
		trader.Context.SetCurrentPrice(kline.Close)
	})

	if err := stream.Connect(ctx); err != nil {
		return nil, err
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		defer stream.Close()

		select {

		case <-ctx.Done():
			return

		}
	}()

	return done, nil
}

func (trader *Trader) reportPnL() {
	report := trader.ProfitAndLossCalculator.Calculate()
	report.Print()
	trader.NotifyPnL(report)
}

func (trader *Trader) NotifyPnL(report *accounting.ProfitAndLossReport) {
	for _, n := range trader.Notifiers {
		n.NotifyPnL(report)
	}
}

func (trader *Trader) NotifyTrade(trade *types.Trade) {
	for _, n := range trader.Notifiers {
		n.NotifyTrade(trade)
	}
}

func (trader *Trader) Notify(msg string, args ...interface{}) {
	for _, n := range trader.Notifiers {
		n.Notify(msg, args...)
	}
}

func (trader *Trader) SubmitOrder(ctx context.Context, order *types.SubmitOrder) {
	trader.Notify(":memo: Submitting %s %s %s order with quantity: %s", order.Symbol, order.Type, order.Side, order.QuantityString, order)

	orderProcessor := &OrderProcessor{
		MinQuoteBalance: 0,
		MaxAssetBalance: 0,
		MinAssetBalance: 0,
		MinProfitSpread: 0,
		MaxOrderAmount:  0,
		Exchange:        trader.Exchange,
		Trader:          trader,
	}

	err := orderProcessor.Submit(ctx, order)

	if err != nil {
		log.WithError(err).Errorf("order create error: side %s quantity: %s", order.Side, order.QuantityString)
		return
	}
}

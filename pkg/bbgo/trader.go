package bbgo

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/accounting"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"

	_ "github.com/go-sql-driver/mysql"
)

// SingleExchangeStrategy represents the single Exchange strategy
type SingleExchangeStrategy interface {
	Run(trader types.Trader, session *ExchangeSession) error
}

type CrossExchangeStrategy interface {
	Run(trader types.Trader, sessions map[string]*ExchangeSession) error
}

// ExchangeSession presents the exchange connection session
// It also maintains and collects the data returned from the stream.
type ExchangeSession struct {
	// Exchange session name
	Name string

	// The exchange account states
	Account *Account

	// Stream is the connection stream of the exchange
	Stream types.Stream

	Subscriptions []types.Subscription

	Exchange types.Exchange

	// Markets defines market configuration of a symbol
	Markets map[string]types.Market

	LastPrices map[string]float64

	// Trades collects the executed trades from the exchange
	// map: symbol -> []trade
	Trades map[string][]types.Trade

	MarketDataStore *MarketDataStore
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

	sessions map[string]*ExchangeSession
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
		Name:       name,
		Exchange:   exchange,
		Markets:    make(map[string]types.Market),
		Trades:     make(map[string][]types.Trade),
		LastPrices: make(map[string]float64),
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

			currentPrice, err := session.Exchange.QueryAveragePrice(ctx, symbol)
			if err != nil {
				return err
			}

			session.LastPrices[symbol] = currentPrice
		}

		session.Account, err = LoadAccount(ctx, session.Exchange)
		if err != nil {
			return err
		}

		session.Stream = session.Exchange.NewStream()

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
				log.WithError(err).Errorf("trade insert error: %+v", *trade)
			}
		})
	}

	return nil
}

func (environ *Environment) Connect(ctx context.Context) error {
	for _, session := range environ.sessions {
		if err := session.Stream.Connect(ctx); err != nil {
			return err
		}
	}

	return nil
}

type Trader struct {
	reportTimer             *time.Timer
	ProfitAndLossCalculator *accounting.ProfitAndLossCalculator

	notifiers   []Notifier
	environment *Environment

	crossExchangeStrategies []CrossExchangeStrategy
	exchangeStrategies      map[string][]SingleExchangeStrategy
}

func NewTrader(environ *Environment) *Trader {
	return &Trader{
		environment:        environ,
		exchangeStrategies: make(map[string][]SingleExchangeStrategy),
	}
}

func (trader *Trader) AddNotifier(notifier Notifier) {
	trader.notifiers = append(trader.notifiers, notifier)
}

// AttachStrategy attaches the single exchange strategy on an exchange session.
// Single exchange strategy is the default behavior.
func (trader *Trader) AttachStrategy(session string, strategy SingleExchangeStrategy) error {
	if _, ok := trader.environment.sessions[session]; !ok {
		return errors.New("session not defined")
	}

	trader.exchangeStrategies[session] = append(trader.exchangeStrategies[session], strategy)
	return nil
}

// AttachCrossExchangeStrategy attaches the cross exchange strategy
func (trader *Trader) AttachCrossExchangeStrategy(strategy CrossExchangeStrategy) error {
	trader.crossExchangeStrategies = append(trader.crossExchangeStrategies, strategy)
	return nil
}

func (trader *Trader) Run(ctx context.Context) error {
	if err := trader.environment.Init(ctx); err != nil {
		return err
	}

	// load and run session strategies
	for session, strategies := range trader.exchangeStrategies {
		for _, strategy := range strategies {
			err := strategy.Run(trader, trader.environment.sessions[session])
			if err != nil {
				return err
			}
		}
	}

	for _, strategy := range trader.crossExchangeStrategies {
		if err := strategy.Run(trader, trader.environment.sessions) ; err != nil {
			return err
		}
	}


	return trader.environment.Connect(ctx)
	/*
		stockManager := &StockDistribution{
			Symbol:             symbol,
			TradingFeeCurrency: tradingFeeCurrency,
		}

		checkpoints, err := stockManager.AddTrades(trades)
		if err != nil {
			return err
		}

		log.Infof("symbol %s: found stock checkpoints: %+v", symbol, checkpoints)
	*/
}

func (trader *Trader) Initialize(ctx context.Context, startTime time.Time) error {
	/*
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
	*/

	/*
		trader.ProfitAndLossCalculator = &accounting.ProfitAndLossCalculator{
			TradingFeeCurrency: tradingFeeCurrency,
			Symbol:             trader.Symbol,
			StartTime:          startTime,
			CurrentPrice:       currentPrice,
			Trades:             trades,
		}
	*/

	// trader.Context.Balances = account.Balances
	// account.Print()
	return nil
}

/*
func (trader *Trader) RunStrategyWithHotReload(ctx context.Context, strategy SingleExchangeStrategy, configFile string) (chan struct{}, error) {
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
*/

/*
func (trader *Trader) RunStrategy(ctx context.Context, strategy SingleExchangeStrategy) (chan struct{}, error) {
	if err := strategy.OnLoad(trader.Context, trader); err != nil {
		return nil, err
	}

	stream := trader.Exchange.NewStream()

	// bind kline store to the stream
	klineStore := NewMarketDataStore()
	klineStore.BindStream(stream)

	trader.Account.BindStream(stream)

	if err := strategy.OnNewStream(stream); err != nil {
		return nil, err
	}

	trader.reportTimer = time.AfterFunc(1*time.Second, func() {
		trader.reportPnL()
	})

	stream.OnTrade(func(trade *types.Trade) {
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
*/

func (trader *Trader) reportPnL() {
	report := trader.ProfitAndLossCalculator.Calculate()
	report.Print()
	trader.NotifyPnL(report)
}

func (trader *Trader) NotifyPnL(report *accounting.ProfitAndLossReport) {
	for _, n := range trader.notifiers {
		n.NotifyPnL(report)
	}
}

func (trader *Trader) NotifyTrade(trade *types.Trade) {
	for _, n := range trader.notifiers {
		n.NotifyTrade(trade)
	}
}

func (trader *Trader) Notify(msg string, args ...interface{}) {
	for _, n := range trader.notifiers {
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
		// FIXME:
		// Exchange:        trader.Exchange,
		Trader: trader,
	}

	err := orderProcessor.Submit(ctx, order)

	if err != nil {
		log.WithError(err).Errorf("order create error: side %s quantity: %s", order.Side, order.QuantityString)
		return
	}
}

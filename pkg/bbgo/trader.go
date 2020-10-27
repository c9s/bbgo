package bbgo

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"

	_ "github.com/go-sql-driver/mysql"
	flag "github.com/spf13/pflag"
)

var SupportedExchanges = []types.ExchangeName{"binance", "max"}

// PersistentFlags defines the flags for environments
func PersistentFlags(flags *flag.FlagSet) {
	flags.String("binance-api-key", "", "binance api key")
	flags.String("binance-api-secret", "", "binance api secret")
	flags.String("max-api-key", "", "max api key")
	flags.String("max-api-secret", "", "max api secret")
}

// SingleExchangeStrategy represents the single Exchange strategy
type SingleExchangeStrategy interface {
	Run(ctx context.Context, orderExecutor OrderExecutor, session *ExchangeSession) error
}

type CrossExchangeStrategy interface {
	Run(ctx context.Context, orderExecutionRouter OrderExecutionRouter, sessions map[string]*ExchangeSession) error
}

type Trader struct {
	Notifiability

	environment *Environment

	riskControls *RiskControls

	crossExchangeStrategies []CrossExchangeStrategy
	exchangeStrategies      map[string][]SingleExchangeStrategy

	tradeReporter *TradeReporter
	// reportTimer             *time.Timer
	// ProfitAndLossCalculator *accounting.ProfitAndLossCalculator
}

func NewTrader(environ *Environment) *Trader {
	return &Trader{
		environment:        environ,
		exchangeStrategies: make(map[string][]SingleExchangeStrategy),
	}
}

func (trader *Trader) ReportTrade() *TradeReporter {
	trader.tradeReporter = NewTradeReporter(&trader.Notifiability)
	return trader.tradeReporter
}


// AttachStrategyOn attaches the single exchange strategy on an exchange session.
// Single exchange strategy is the default behavior.
func (trader *Trader) AttachStrategyOn(session string, strategies ...SingleExchangeStrategy) *Trader {
	if _, ok := trader.environment.sessions[session]; !ok {
		log.Panicf("session %s is not defined", session)
	}

	for _, s := range strategies {
		trader.exchangeStrategies[session] = append(trader.exchangeStrategies[session], s)
	}

	return trader
}

// AttachCrossExchangeStrategy attaches the cross exchange strategy
func (trader *Trader) AttachCrossExchangeStrategy(strategy CrossExchangeStrategy) *Trader {
	trader.crossExchangeStrategies = append(trader.crossExchangeStrategies, strategy)

	return trader
}

// TODO: provide a more DSL way to configure risk controls
func (trader *Trader) SetRiskControls(riskControls *RiskControls) {
	trader.riskControls = riskControls
}

func (trader *Trader) Run(ctx context.Context) error {
	if err := trader.environment.Init(ctx); err != nil {
		return err
	}

	// load and run session strategies
	for sessionName, strategies := range trader.exchangeStrategies {
		session := trader.environment.sessions[sessionName]

		if session.tradeReporter != nil {
			session.Stream.OnTrade(func(trade types.Trade) {
				session.tradeReporter.Report(trade)
			})
		} else if trader.tradeReporter != nil {
			session.Stream.OnTrade(func(trade types.Trade) {
				trader.tradeReporter.Report(trade)
			})
		}


		// We can move this to the exchange session,
		// that way we can mount the notification on the exchange with DSL
		// This is the default order executor
		var baseOrderExecutor = &ExchangeOrderExecutor{
			Notifiability: trader.Notifiability,
			session:       session,
		}

		// default to base order executor
		var orderExecutor OrderExecutor = baseOrderExecutor

		// Since the risk controls are loaded from the config file
		if riskControls := trader.riskControls ; riskControls != nil {
			if trader.riskControls.SessionBasedRiskControl != nil {
				control, ok := trader.riskControls.SessionBasedRiskControl[sessionName]
				if ok {
					control.SetBaseOrderExecutor(baseOrderExecutor)

					// pick the order executor
					if control.OrderExecutor != nil {
						orderExecutor = control.OrderExecutor
					}
				}
			}
		}

		for _, strategy := range strategies {
			err := strategy.Run(ctx, orderExecutor, session)
			if err != nil {
				return err
			}
		}
	}

	router := &ExchangeOrderExecutionRouter{
		// copy the parent notifiers
		Notifiability: trader.Notifiability,
		sessions:      trader.environment.sessions,
	}

	for _, strategy := range trader.crossExchangeStrategies {
		if err := strategy.Run(ctx, router, trader.environment.sessions); err != nil {
			return err
		}
	}

	return trader.environment.Connect(ctx)
}

/*
func (trader *OrderExecutor) RunStrategyWithHotReload(ctx context.Context, strategy SingleExchangeStrategy, configFile string) (chan struct{}, error) {
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

				trader.NotifyTo("config reloaded, restarting trader")

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
func (trader *OrderExecutor) RunStrategy(ctx context.Context, strategy SingleExchangeStrategy) (chan struct{}, error) {
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
}
*/

/*
func (trader *Trader) reportPnL() {
	report := trader.ProfitAndLossCalculator.Calculate()
	report.Print()
	trader.NotifyPnL(report)
}
*/

// ReportPnL configure and set the PnLReporter with the given notifier
func (trader *Trader) ReportPnL(notifier Notifier) *PnLReporterManager {
	return NewPnLReporter(notifier)
}

type OrderExecutor interface {
	SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders []types.Order, err error)
}

type OrderExecutionRouter interface {
	// SubmitOrderTo submit order to a specific exchange session
	SubmitOrdersTo(ctx context.Context, session string, orders ...types.SubmitOrder) (createdOrders []types.Order, err error)
}

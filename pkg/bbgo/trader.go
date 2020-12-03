package bbgo

import (
	"context"
	"reflect"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"

	_ "github.com/go-sql-driver/mysql"
)

var SupportedExchanges = []types.ExchangeName{"binance", "max"}

// SingleExchangeStrategy represents the single Exchange strategy
type SingleExchangeStrategy interface {
	Run(ctx context.Context, orderExecutor OrderExecutor, session *ExchangeSession) error
}

type ExchangeSessionSubscriber interface {
	Subscribe(session *ExchangeSession)
}

type CrossExchangeSessionSubscriber interface {
	CrossSubscribe(sessions map[string]*ExchangeSession)
}

type CrossExchangeStrategy interface {
	CrossRun(ctx context.Context, orderExecutionRouter OrderExecutionRouter, sessions map[string]*ExchangeSession) error
}

//go:generate callbackgen -type Graceful
type Graceful struct {
	shutdownCallbacks []func(ctx context.Context, wg *sync.WaitGroup)
}

func (g *Graceful) Shutdown(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(len(g.shutdownCallbacks))

	go g.EmitShutdown(ctx, &wg)

	wg.Wait()
}

type Logging interface {
	EnableLogging()
	DisableLogging()
}

type Logger interface {
	Warnf(message string, args ...interface{})
	Errorf(message string, args ...interface{})
	Infof(message string, args ...interface{})
}

type SilentLogger struct{}

func (logger *SilentLogger) Infof(message string, args ...interface{})  {}
func (logger *SilentLogger) Warnf(message string, args ...interface{})  {}
func (logger *SilentLogger) Errorf(message string, args ...interface{}) {}

type Trader struct {
	environment *Environment

	riskControls *RiskControls

	crossExchangeStrategies []CrossExchangeStrategy
	exchangeStrategies      map[string][]SingleExchangeStrategy

	logger Logger

	Graceful Graceful
}

func NewTrader(environ *Environment) *Trader {
	return &Trader{
		environment:        environ,
		exchangeStrategies: make(map[string][]SingleExchangeStrategy),
		logger:             log.StandardLogger(),
	}
}

func (trader *Trader) EnableLogging() {
	trader.logger = log.StandardLogger()
}

func (trader *Trader) DisableLogging() {
	trader.logger = &SilentLogger{}
}

// AttachStrategyOn attaches the single exchange strategy on an exchange Session.
// Single exchange strategy is the default behavior.
func (trader *Trader) AttachStrategyOn(session string, strategies ...SingleExchangeStrategy) *Trader {
	if _, ok := trader.environment.sessions[session]; !ok {
		log.Panicf("Session %s is not defined", session)
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
	// pre-subscribe the data
	for sessionName, strategies := range trader.exchangeStrategies {
		session := trader.environment.sessions[sessionName]
		for _, strategy := range strategies {
			if subscriber, ok := strategy.(ExchangeSessionSubscriber); ok {
				subscriber.Subscribe(session)
			}
		}
	}

	for _, strategy := range trader.crossExchangeStrategies {
		if subscriber, ok := strategy.(CrossExchangeSessionSubscriber); ok {
			subscriber.CrossSubscribe(trader.environment.sessions)
		}
	}

	if err := trader.environment.Init(ctx); err != nil {
		return err
	}

	// load and run Session strategies
	for sessionName, strategies := range trader.exchangeStrategies {
		var session = trader.environment.sessions[sessionName]

		var baseOrderExecutor = &ExchangeOrderExecutor{
			// copy the environment notification system so that we can route
			Notifiability: trader.environment.Notifiability,
			Session:       session,
		}

		// default to base order executor
		var orderExecutor OrderExecutor = baseOrderExecutor

		// Since the risk controls are loaded from the config file
		if riskControls := trader.riskControls; riskControls != nil {
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
			rs := reflect.ValueOf(strategy)
			if rs.Elem().Kind() == reflect.Struct {
				// get the struct element
				rs = rs.Elem()

				if err := injectField(rs, "Graceful", &trader.Graceful, true); err != nil {
					log.WithError(err).Errorf("strategy Graceful injection failed")
				}

				if err := injectField(rs, "Logger", &trader.logger, false); err != nil {
					log.WithError(err).Errorf("strategy Logger injection failed")
				}

				if err := injectField(rs, "Notifiability", &trader.environment.Notifiability, false); err != nil {
					log.WithError(err).Errorf("strategy Notifiability injection failed")
				}

				if err := injectField(rs, "OrderExecutor", orderExecutor, false); err != nil {
					log.WithError(err).Errorf("strategy OrderExecutor injection failed")
				}

				if symbol, ok := isSymbolBasedStrategy(rs); ok {
					log.Infof("found symbol based strategy from %s", rs.Type())
					if hasField(rs, "Market") {
						if market, ok := session.Market(symbol); ok {
							// let's make the market object passed by pointer
							if err := injectField(rs, "Market", &market, false); err != nil {
								log.WithError(err).Errorf("strategy %T Market injection failed", strategy)
							}
						}
					}

					// StandardIndicatorSet
					if hasField(rs, "StandardIndicatorSet") {
						if indicatorSet, ok := session.StandardIndicatorSet(symbol); ok {
							if err := injectField(rs, "StandardIndicatorSet", indicatorSet, true); err != nil {
								log.WithError(err).Errorf("strategy %T StandardIndicatorSet injection failed", strategy)
							}
						}
					}

					if hasField(rs, "MarketDataStore") {
						if store, ok := session.MarketDataStore(symbol); ok {
							if err := injectField(rs, "MarketDataStore", store, true); err != nil {
								log.WithError(err).Errorf("strategy %T MarketDataStore injection failed", strategy)
							}
						}
					}
				}
			}

			err := strategy.Run(ctx, orderExecutor, session)
			if err != nil {
				return err
			}
		}
	}

	router := &ExchangeOrderExecutionRouter{
		Notifiability: trader.environment.Notifiability,
		sessions:      trader.environment.sessions,
	}

	for _, strategy := range trader.crossExchangeStrategies {
		rs := reflect.ValueOf(strategy)
		if rs.Elem().Kind() == reflect.Struct {
			// get the struct element
			rs = rs.Elem()

			if err := injectField(rs, "Graceful", &trader.Graceful, true); err != nil {
				log.WithError(err).Errorf("strategy Graceful injection failed")
			}

			if err := injectField(rs, "Logger", &trader.logger, false); err != nil {
				log.WithError(err).Errorf("strategy Logger injection failed")
			}

			if err := injectField(rs, "Notifiability", &trader.environment.Notifiability, false); err != nil {
				log.WithError(err).Errorf("strategy Notifiability injection failed")
			}

		}

		if err := strategy.CrossRun(ctx, router, trader.environment.sessions); err != nil {
			return err
		}
	}

	return trader.environment.Connect(ctx)
}

// ReportPnL configure and set the PnLReporter with the given notifier
func (trader *Trader) ReportPnL() *PnLReporterManager {
	return NewPnLReporter(&trader.environment.Notifiability)
}

type OrderExecutor interface {
	SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error)
}

type OrderExecutionRouter interface {
	// SubmitOrderTo submit order to a specific exchange Session
	SubmitOrdersTo(ctx context.Context, session string, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error)
}

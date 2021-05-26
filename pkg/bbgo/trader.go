package bbgo

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	_ "github.com/go-sql-driver/mysql"
)

// SingleExchangeStrategy represents the single Exchange strategy
type SingleExchangeStrategy interface {
	ID() string
	Run(ctx context.Context, orderExecutor OrderExecutor, session *ExchangeSession) error
}

type ExchangeSessionSubscriber interface {
	Subscribe(session *ExchangeSession)
}

type CrossExchangeSessionSubscriber interface {
	CrossSubscribe(sessions map[string]*ExchangeSession)
}

type CrossExchangeStrategy interface {
	ID() string
	CrossRun(ctx context.Context, orderExecutionRouter OrderExecutionRouter, sessions map[string]*ExchangeSession) error
}

type Validator interface {
	Validate() error
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

func (logger *SilentLogger) Infof(string, ...interface{})  {}
func (logger *SilentLogger) Warnf(string, ...interface{})  {}
func (logger *SilentLogger) Errorf(string, ...interface{}) {}

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

func (trader *Trader) Configure(userConfig *Config) error {
	if userConfig.RiskControls != nil {
		trader.SetRiskControls(userConfig.RiskControls)
	}

	for _, entry := range userConfig.ExchangeStrategies {
		for _, mount := range entry.Mounts {
			log.Infof("attaching strategy %T on %s...", entry.Strategy, mount)
			if err := trader.AttachStrategyOn(mount, entry.Strategy); err != nil {
				return err
			}
		}
	}

	for _, strategy := range userConfig.CrossExchangeStrategies {
		log.Infof("attaching cross exchange strategy %T", strategy)
		trader.AttachCrossExchangeStrategy(strategy)
	}

	for _, report := range userConfig.PnLReporters {
		if len(report.AverageCostBySymbols) > 0 {

			log.Infof("setting up average cost pnl reporter on symbols: %v", report.AverageCostBySymbols)
			trader.ReportPnL().
				AverageCostBySymbols(report.AverageCostBySymbols...).
				Of(report.Of...).
				When(report.When...)

		} else {
			return fmt.Errorf("unsupported PnL reporter: %+v", report)
		}
	}

	return nil
}

// AttachStrategyOn attaches the single exchange strategy on an exchange Session.
// Single exchange strategy is the default behavior.
func (trader *Trader) AttachStrategyOn(session string, strategies ...SingleExchangeStrategy) error {
	if len(trader.environment.sessions) == 0 {
		return fmt.Errorf("you don't have any session configured, please check your environment variable or config file")
	}

	if _, ok := trader.environment.sessions[session]; !ok {
		var keys []string
		for k := range trader.environment.sessions {
			keys = append(keys, k)
		}

		return fmt.Errorf("session %s is not defined, valid sessions are: %v", session, keys)
	}

	for _, s := range strategies {
		trader.exchangeStrategies[session] = append(trader.exchangeStrategies[session], s)
	}

	return nil
}

// AttachCrossExchangeStrategy attaches the cross exchange strategy
func (trader *Trader) AttachCrossExchangeStrategy(strategy CrossExchangeStrategy) *Trader {
	trader.crossExchangeStrategies = append(trader.crossExchangeStrategies, strategy)

	return trader
}

// SetRiskControls sets the risk controller
// TODO: provide a more DSL way to configure risk controls
func (trader *Trader) SetRiskControls(riskControls *RiskControls) {
	trader.riskControls = riskControls
}

func (trader *Trader) Subscribe() {
	// pre-subscribe the data
	for sessionName, strategies := range trader.exchangeStrategies {
		session := trader.environment.sessions[sessionName]
		for _, strategy := range strategies {
			if subscriber, ok := strategy.(ExchangeSessionSubscriber); ok {
				subscriber.Subscribe(session)
			} else {
				log.Errorf("strategy %s does not implement ExchangeSessionSubscriber", strategy.ID())
			}
		}
	}

	for _, strategy := range trader.crossExchangeStrategies {
		if subscriber, ok := strategy.(CrossExchangeSessionSubscriber); ok {
			subscriber.CrossSubscribe(trader.environment.sessions)
		} else {
			log.Errorf("strategy %s does not implement CrossExchangeSessionSubscriber", strategy.ID())
		}
	}
}

func (trader *Trader) RunSingleExchangeStrategy(ctx context.Context, strategy SingleExchangeStrategy, session *ExchangeSession, orderExecutor OrderExecutor) error {
	rs := reflect.ValueOf(strategy)

	// get the struct element
	rs = rs.Elem()

	if rs.Kind() != reflect.Struct {
		return errors.New("strategy object is not a struct")
	}

	if err := trader.injectCommonServices(rs); err != nil {
		return err
	}

	if err := injectField(rs, "OrderExecutor", orderExecutor, false); err != nil {
		return errors.Wrapf(err, "failed to inject OrderExecutor on %T", strategy)
	}

	if symbol, ok := isSymbolBasedStrategy(rs); ok {
		log.Infof("found symbol based strategy from %s", rs.Type())
		if _, ok := hasField(rs, "Market"); ok {
			if market, ok := session.Market(symbol); ok {
				// let's make the market object passed by pointer
				if err := injectField(rs, "Market", &market, false); err != nil {
					return errors.Wrapf(err, "failed to inject Market on %T", strategy)
				}
			}
		}

		// StandardIndicatorSet
		if _, ok := hasField(rs, "StandardIndicatorSet"); ok {
			indicatorSet, ok := session.StandardIndicatorSet(symbol)
			if !ok {
				return fmt.Errorf("standardIndicatorSet of symbol %s not found", symbol)
			}

			if err := injectField(rs, "StandardIndicatorSet", indicatorSet, true); err != nil {
				return errors.Wrapf(err, "failed to inject StandardIndicatorSet on %T", strategy)
			}
		}

		if _, ok := hasField(rs, "MarketDataStore"); ok {
			store, ok := session.MarketDataStore(symbol)
			if !ok {
				return fmt.Errorf("marketDataStore of symbol %s not found", symbol)
			}

			if err := injectField(rs, "MarketDataStore", store, true); err != nil {
				return errors.Wrapf(err, "failed to inject MarketDataStore on %T", strategy)
			}
		}
	}

	// If the strategy has Validate() method, run it and check the error
	if v, ok := strategy.(Validator); ok {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("failed to validate the config: %w", err)
		}
	}

	return strategy.Run(ctx, orderExecutor, session)
}

func (trader *Trader) getSessionOrderExecutor(sessionName string) OrderExecutor {
	var session = trader.environment.sessions[sessionName]

	// default to base order executor
	var orderExecutor OrderExecutor = session.OrderExecutor

	// Since the risk controls are loaded from the config file
	if trader.riskControls != nil && trader.riskControls.SessionBasedRiskControl != nil {
		if control, ok := trader.riskControls.SessionBasedRiskControl[sessionName]; ok {
			control.SetBaseOrderExecutor(session.OrderExecutor)

			// pick the wrapped order executor
			if control.OrderExecutor != nil {
				return control.OrderExecutor
			}
		}
	}

	return orderExecutor
}

func (trader *Trader) RunAllSingleExchangeStrategy(ctx context.Context) error {
	// load and run Session strategies
	for sessionName, strategies := range trader.exchangeStrategies {
		var session = trader.environment.sessions[sessionName]
		var orderExecutor = trader.getSessionOrderExecutor(sessionName)
		for _, strategy := range strategies {
			if err := trader.RunSingleExchangeStrategy(ctx, strategy, session, orderExecutor); err != nil {
				return err
			}
		}
	}

	return nil
}

func (trader *Trader) Run(ctx context.Context) error {
	trader.Subscribe()

	if err := trader.environment.Start(ctx); err != nil {
		return err
	}

	if err := trader.RunAllSingleExchangeStrategy(ctx); err != nil {
		return err
	}

	router := &ExchangeOrderExecutionRouter{
		Notifiability: trader.environment.Notifiability,
		sessions:      trader.environment.sessions,
		executors:     make(map[string]OrderExecutor),
	}
	for sessionID := range trader.environment.sessions {
		var orderExecutor = trader.getSessionOrderExecutor(sessionID)
		router.executors[sessionID] = orderExecutor
	}

	for _, strategy := range trader.crossExchangeStrategies {
		rs := reflect.ValueOf(strategy)

		// get the struct element from the struct pointer
		rs = rs.Elem()

		if rs.Kind() != reflect.Struct {
			continue
		}

		if err := trader.injectCommonServices(rs); err != nil {
			return err
		}

		if err := strategy.CrossRun(ctx, router, trader.environment.sessions); err != nil {
			return err
		}
	}

	return trader.environment.Connect(ctx)
}

func (trader *Trader) injectCommonServices(rs reflect.Value) error {
	if err := injectField(rs, "Graceful", &trader.Graceful, true); err != nil {
		return errors.Wrap(err, "failed to inject Graceful")
	}

	if err := injectField(rs, "Logger", &trader.logger, false); err != nil {
		return errors.Wrap(err, "failed to inject Logger")
	}

	if err := injectField(rs, "Notifiability", &trader.environment.Notifiability, false); err != nil {
		return errors.Wrap(err, "failed to inject Notifiability")
	}

	if trader.environment.TradeService != nil {
		if err := injectField(rs, "TradeService", trader.environment.TradeService, true); err != nil {
			return errors.Wrap(err, "failed to inject TradeService")
		}
	}

	if field, ok := hasField(rs, "Persistence"); ok {
		if trader.environment.PersistenceServiceFacade == nil {
			log.Warnf("strategy has Persistence field but persistence service is not defined")
		} else {
			if field.IsNil() {
				field.Set(reflect.ValueOf(&Persistence{
					PersistenceSelector: &PersistenceSelector{
						StoreID: "default",
						Type:    "memory",
					},
					Facade: trader.environment.PersistenceServiceFacade,
				}))
			} else {
				elem := field.Elem()
				if elem.Kind() != reflect.Struct {
					return fmt.Errorf("field Persistence is not a struct element")
				}

				if err := injectField(elem, "Facade", trader.environment.PersistenceServiceFacade, true); err != nil {
					return errors.Wrap(err, "failed to inject Persistence")
				}
			}
		}
	}

	return nil
}

// ReportPnL configure and set the PnLReporter with the given notifier
func (trader *Trader) ReportPnL() *PnLReporterManager {
	return NewPnLReporter(&trader.environment.Notifiability)
}

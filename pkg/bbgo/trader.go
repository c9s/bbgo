package bbgo

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	_ "github.com/go-sql-driver/mysql"

	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/interact"
)

// Strategy method calls:
// -> Initialize()   (optional method)
// -> Defaults()     (optional method)
// -> Validate()     (optional method)
// -> Run()          (optional method)
// -> Shutdown(shutdownCtx context.Context, wg *sync.WaitGroup)
type StrategyID interface {
	ID() string
}

// SingleExchangeStrategy represents the single Exchange strategy
type SingleExchangeStrategy interface {
	StrategyID
	Run(ctx context.Context, orderExecutor OrderExecutor, session *ExchangeSession) error
}

// StrategyInitializer's Initialize method is called before the Subscribe method call.
type StrategyInitializer interface {
	Initialize() error
}

type StrategyDefaulter interface {
	Defaults() error
}

type StrategyValidator interface {
	Validate() error
}

type StrategyShutdown interface {
	Shutdown(ctx context.Context, wg *sync.WaitGroup)
}

// ExchangeSessionSubscriber provides an interface for collecting subscriptions from different strategies
// Subscribe method will be called before the user data stream connection is created.
type ExchangeSessionSubscriber interface {
	Subscribe(session *ExchangeSession)
}

type CrossExchangeSessionSubscriber interface {
	CrossSubscribe(sessions map[string]*ExchangeSession)
}

type CrossExchangeStrategy interface {
	StrategyID
	CrossRun(ctx context.Context, orderExecutionRouter OrderExecutionRouter, sessions map[string]*ExchangeSession) error
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

	// gracefulShutdown is used for registering strategy's Shutdown calls
	// when strategy implements Shutdown(ctx), the func ref will be stored in the callback.
	gracefulShutdown GracefulShutdown

	logger Logger
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

	trader.exchangeStrategies[session] = append(
		trader.exchangeStrategies[session], strategies...)

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

func (trader *Trader) RunSingleExchangeStrategy(
	ctx context.Context, strategy SingleExchangeStrategy, session *ExchangeSession, orderExecutor OrderExecutor,
) error {
	if v, ok := strategy.(StrategyValidator); ok {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("failed to validate the config: %w", err)
		}
	}

	if shutdown, ok := strategy.(StrategyShutdown); ok {
		trader.gracefulShutdown.OnShutdown(shutdown.Shutdown)
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

func (trader *Trader) injectFieldsAndSubscribe(ctx context.Context) error {
	// load and run Session strategies
	for sessionName, strategies := range trader.exchangeStrategies {
		var session = trader.environment.sessions[sessionName]
		for _, strategy := range strategies {
			rs := reflect.ValueOf(strategy)

			// get the struct element
			rs = rs.Elem()

			if rs.Kind() != reflect.Struct {
				return errors.New("strategy object is not a struct")
			}

			if err := trader.injectCommonServices(ctx, strategy); err != nil {
				return err
			}

			if defaulter, ok := strategy.(StrategyDefaulter); ok {
				if err := defaulter.Defaults(); err != nil {
					panic(err)
				}
			}

			if subscriber, ok := strategy.(ExchangeSessionSubscriber); ok {
				subscriber.Subscribe(session)
			} else {
				log.Errorf("strategy %s does not implement ExchangeSessionSubscriber", strategy.ID())
			}

			if symbol, ok := dynamic.LookupSymbolField(rs); ok && symbol != "" {
				log.Infof("found symbol %s based strategy from %s", symbol, rs.Type())

				if err := session.initSymbol(ctx, trader.environment, symbol); err != nil {
					return errors.Wrapf(err, "failed to inject object into %T when initSymbol", strategy)
				}

				market, ok := session.Market(symbol)
				if !ok {
					return fmt.Errorf("market of symbol %s not found", symbol)
				}

				indicatorSet := session.StandardIndicatorSet(symbol)
				if !ok {
					return fmt.Errorf("standardIndicatorSet of symbol %s not found", symbol)
				}

				store, ok := session.MarketDataStore(symbol)
				if !ok {
					return fmt.Errorf("marketDataStore of symbol %s not found", symbol)
				}

				if err := dynamic.ParseStructAndInject(strategy,
					market,
					session,
					session.OrderExecutor,
					indicatorSet,
					store,
				); err != nil {
					return errors.Wrapf(err, "failed to inject object into %T", strategy)
				}
			}
		}
	}

	for _, strategy := range trader.crossExchangeStrategies {
		rs := reflect.ValueOf(strategy)

		// get the struct element from the struct pointer
		rs = rs.Elem()
		if rs.Kind() != reflect.Struct {
			continue
		}

		if err := trader.injectCommonServices(ctx, strategy); err != nil {
			return err
		}

		if defaulter, ok := strategy.(StrategyDefaulter); ok {
			if err := defaulter.Defaults(); err != nil {
				return err
			}
		}

		if initializer, ok := strategy.(StrategyInitializer); ok {
			if err := initializer.Initialize(); err != nil {
				return err
			}
		}

		if subscriber, ok := strategy.(CrossExchangeSessionSubscriber); ok {
			subscriber.CrossSubscribe(trader.environment.sessions)
		} else {
			log.Errorf("strategy %s does not implement CrossExchangeSessionSubscriber", strategy.ID())
		}
	}

	return nil
}

func (trader *Trader) Run(ctx context.Context) error {
	// before we start the interaction,
	// register the core interaction, because we can only get the strategies in this scope
	// trader.environment.Connect will call interact.Start
	interact.AddCustomInteraction(NewCoreInteraction(trader.environment, trader))

	if err := trader.injectFieldsAndSubscribe(ctx); err != nil {
		return err
	}

	if err := trader.environment.Start(ctx); err != nil {
		return err
	}

	if err := trader.RunAllSingleExchangeStrategy(ctx); err != nil {
		return err
	}

	router := &ExchangeOrderExecutionRouter{
		sessions:  trader.environment.sessions,
		executors: make(map[string]OrderExecutor),
	}
	for sessionID := range trader.environment.sessions {
		var orderExecutor = trader.getSessionOrderExecutor(sessionID)
		router.executors[sessionID] = orderExecutor
	}

	for _, strategy := range trader.crossExchangeStrategies {
		if err := strategy.CrossRun(ctx, router, trader.environment.sessions); err != nil {
			return err
		}
	}

	return trader.environment.Connect(ctx)
}

func (trader *Trader) Initialize(ctx context.Context) error {
	return trader.IterateStrategies(func(strategy StrategyID) error {
		if initializer, ok := strategy.(StrategyInitializer); ok {
			return initializer.Initialize()
		}

		return nil
	})
}

func (trader *Trader) LoadState(ctx context.Context) error {
	if trader.environment.BacktestService != nil {
		return nil
	}

	isolation := GetIsolationFromContext(ctx)
	ps := isolation.persistenceServiceFacade.Get()

	log.Infof("loading strategies states...")
	return trader.IterateStrategies(func(strategy StrategyID) error {
		id := dynamic.CallID(strategy)
		return loadPersistenceFields(strategy, id, ps)
	})
}

func (trader *Trader) IterateStrategies(f func(st StrategyID) error) error {
	for _, strategies := range trader.exchangeStrategies {
		for _, strategy := range strategies {
			if err := f(strategy); err != nil {
				return err
			}
		}
	}

	for _, strategy := range trader.crossExchangeStrategies {
		if err := f(strategy); err != nil {
			return err
		}
	}

	return nil
}

// NOTICE: the ctx here is the trading context, which could already be canceled.
func (trader *Trader) SaveState(ctx context.Context) error {
	if trader.environment.BacktestService != nil {
		return nil
	}

	isolation := GetIsolationFromContext(ctx)

	ps := isolation.persistenceServiceFacade.Get()

	log.Debugf("saving strategy persistence states...")
	return trader.IterateStrategies(func(strategy StrategyID) error {
		id := dynamic.CallID(strategy)
		if len(id) == 0 {
			return nil
		}

		return storePersistenceFields(strategy, id, ps)
	})
}

func (trader *Trader) Shutdown(ctx context.Context) {
	trader.gracefulShutdown.Shutdown(ctx)
}

func (trader *Trader) injectCommonServices(ctx context.Context, s interface{}) error {
	isolation := GetIsolationFromContext(ctx)

	ps := isolation.persistenceServiceFacade

	// a special injection for persistence selector:
	// if user defined the selector, the facade pointer will be nil, hence we need to update the persistence facade pointer
	sv := reflect.ValueOf(s).Elem()
	if field, ok := dynamic.HasField(sv, "Persistence"); ok {
		// the selector is set, but we need to update the facade pointer
		if !field.IsNil() {
			elem := field.Elem()
			if elem.Kind() != reflect.Struct {
				return fmt.Errorf("field Persistence is not a struct element, %s given", field)
			}

			if err := dynamic.InjectField(elem.Interface(), "Facade", ps, true); err != nil {
				return err
			}

			/*
				if err := ParseStructAndInject(field.Interface(), persistenceFacade); err != nil {
					return err
				}
			*/
		}
	}

	return dynamic.ParseStructAndInject(s,
		&trader.logger,
		Notification,
		trader.environment.TradeService,
		trader.environment.OrderService,
		trader.environment.DatabaseService,
		trader.environment.AccountService,
		trader.environment,
		ps, // if the strategy use persistence facade separately
	)
}

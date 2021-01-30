package bbgo

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/codingconcepts/env"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var LoadedExchangeStrategies = make(map[string]SingleExchangeStrategy)
var LoadedCrossExchangeStrategies = make(map[string]CrossExchangeStrategy)

func RegisterStrategy(key string, s interface{}) {
	loaded := 0
	if d, ok := s.(SingleExchangeStrategy); ok {
		LoadedExchangeStrategies[key] = d
		loaded++
	}

	if d, ok := s.(CrossExchangeStrategy); ok {
		LoadedCrossExchangeStrategies[key] = d
		loaded++
	}

	if loaded == 0 {
		panic(fmt.Errorf("%T does not implement SingleExchangeStrategy or CrossExchangeStrategy", s))
	}
}

var emptyTime time.Time

// Environment presents the real exchange data layer
type Environment struct {
	// Notifiability here for environment is for the streaming data notification
	// note that, for back tests, we don't need notification.
	Notifiability

	PersistenceServiceFacade *PersistenceServiceFacade

	OrderService *service.OrderService
	TradeService *service.TradeService
	TradeSync    *service.SyncService

	// startTime is the time of start point (which is used in the backtest)
	startTime     time.Time
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

func (environ *Environment) Session(name string) (*ExchangeSession, bool) {
	s, ok := environ.sessions[name]
	return s, ok
}

func (environ *Environment) Sessions() map[string]*ExchangeSession {
	return environ.sessions
}

func (environ *Environment) ConfigureDatabase(ctx context.Context) error {
	if viper.IsSet("mysql-url") {
		dsn := viper.GetString("mysql-url")
		db, err := ConnectMySQL(dsn)
		if err != nil {
			return err
		}

		if err := upgradeDB(ctx, "mysql", db.DB); err != nil {
			return err
		}

		environ.SetDB(db)
	}

	return nil
}

func (environ *Environment) SetDB(db *sqlx.DB) *Environment {
	environ.OrderService = &service.OrderService{DB: db}
	environ.TradeService = &service.TradeService{DB: db}
	environ.TradeSync = &service.SyncService{
		TradeService: environ.TradeService,
		OrderService: environ.OrderService,
	}

	return environ
}

// AddExchangeSession adds the existing exchange session or pre-created exchange session
func (environ *Environment) AddExchangeSession(name string, session *ExchangeSession) *ExchangeSession {
	environ.sessions[name] = session
	return session
}

// AddExchange adds the given exchange with the session name, this is the default
func (environ *Environment) AddExchange(name string, exchange types.Exchange) (session *ExchangeSession) {
	session = NewExchangeSession(name, exchange)
	return environ.AddExchangeSession(name, session)
}

func (environ *Environment) AddExchangesFromConfig(userConfig *Config) error {
	if len(userConfig.Sessions) == 0 {
		return environ.AddExchangesByViperKeys()
	}

	return environ.AddExchangesFromSessionConfig(userConfig.Sessions)
}

func (environ *Environment) AddExchangesByViperKeys() error {
	for _, n := range SupportedExchanges {
		if viper.IsSet(string(n) + "-api-key") {
			exchange, err := cmdutil.NewExchangeWithEnvVarPrefix(n, "")
			if err != nil {
				return err
			}

			environ.AddExchange(n.String(), exchange)
		}
	}

	return nil
}

func (environ *Environment) AddExchangesFromSessionConfig(sessions map[string]Session) error {
	for sessionName, sessionConfig := range sessions {
		exchangeName, err := types.ValidExchangeName(sessionConfig.ExchangeName)
		if err != nil {
			return err
		}

		exchange, err := cmdutil.NewExchangeWithEnvVarPrefix(exchangeName, sessionConfig.EnvVarPrefix)
		if err != nil {
			return err
		}

		// configure exchange
		if sessionConfig.Margin {
			marginExchange, ok := exchange.(types.MarginExchange)
			if !ok {
				return fmt.Errorf("exchange %s does not support margin", exchangeName)
			}

			if sessionConfig.IsolatedMargin {
				marginExchange.UseIsolatedMargin(sessionConfig.IsolatedMarginSymbol)
			} else {
				marginExchange.UseMargin()
			}
		}

		session := NewExchangeSession(sessionName, exchange)
		session.IsMargin = sessionConfig.Margin
		session.IsIsolatedMargin = sessionConfig.IsolatedMargin
		session.IsolatedMarginSymbol = sessionConfig.IsolatedMarginSymbol
		environ.AddExchangeSession(sessionName, session)
	}

	return nil
}

// Init prepares the data that will be used by the strategies
func (environ *Environment) Init(ctx context.Context) (err error) {
	// feed klines into the market data store
	if environ.startTime == emptyTime {
		environ.startTime = time.Now()
	}

	for n := range environ.sessions {
		var session = environ.sessions[n]

		if err := session.Init(ctx, environ); err != nil {
			return err
		}

		if err := session.InitSymbols(ctx, environ); err != nil {
			return err
		}

		session.IsInitialized = true
	}

	return nil
}

func (environ *Environment) ConfigurePersistence(conf *PersistenceConfig) error {
	var facade = &PersistenceServiceFacade{
		Memory: NewMemoryService(),
	}

	if conf.Redis != nil {
		if err := env.Set(conf.Redis); err != nil {
			return err
		}

		facade.Redis = NewRedisPersistenceService(conf.Redis)
	}

	if conf.Json != nil {
		if _, err := os.Stat(conf.Json.Directory); os.IsNotExist(err) {
			if err2 := os.MkdirAll(conf.Json.Directory, 0777); err2 != nil {
				log.WithError(err2).Errorf("can not create directory: %s", conf.Json.Directory)
				return err2
			}
		}

		facade.Json = &JsonPersistenceService{Directory: conf.Json.Directory}
	}

	environ.PersistenceServiceFacade = facade
	return nil
}

// configure notification rules
// for symbol-based routes, we should register the same symbol rules for each session.
// for session-based routes, we should set the fixed callbacks for each session
func (environ *Environment) ConfigureNotification(conf *NotificationConfig) error {
	// configure routing here
	if conf.SymbolChannels != nil {
		environ.SymbolChannelRouter.AddRoute(conf.SymbolChannels)
	}
	if conf.SessionChannels != nil {
		environ.SessionChannelRouter.AddRoute(conf.SessionChannels)
	}

	if conf.Routing != nil {
		// configure passive object notification routing
		switch conf.Routing.Trade {
		case "$silent": // silent, do not setup notification

		case "$session":
			defaultTradeUpdateHandler := func(trade types.Trade) {
				text := util.Render(TemplateTradeReport, trade)
				environ.Notify(text, &trade)
			}
			for name := range environ.sessions {
				session := environ.sessions[name]

				// if we can route session name to channel successfully...
				channel, ok := environ.SessionChannelRouter.Route(name)
				if ok {
					session.Stream.OnTradeUpdate(func(trade types.Trade) {
						text := util.Render(TemplateTradeReport, trade)
						environ.NotifyTo(channel, text, &trade)
					})
				} else {
					session.Stream.OnTradeUpdate(defaultTradeUpdateHandler)
				}
			}

		case "$symbol":
			// configure object routes for Trade
			environ.ObjectChannelRouter.Route(func(obj interface{}) (channel string, ok bool) {
				trade, matched := obj.(*types.Trade)
				if !matched {
					return
				}
				channel, ok = environ.SymbolChannelRouter.Route(trade.Symbol)
				return
			})

			// use same handler for each session
			handler := func(trade types.Trade) {
				text := util.Render(TemplateTradeReport, trade)
				channel, ok := environ.RouteObject(&trade)
				if ok {
					environ.NotifyTo(channel, text, &trade)
				} else {
					environ.Notify(text, &trade)
				}
			}
			for _, session := range environ.sessions {
				session.Stream.OnTradeUpdate(handler)
			}
		}

		switch conf.Routing.Order {

		case "$silent": // silent, do not setup notification

		case "$session":
			defaultOrderUpdateHandler := func(order types.Order) {
				text := util.Render(TemplateOrderReport, order)
				environ.Notify(text, &order)
			}
			for name := range environ.sessions {
				session := environ.sessions[name]

				// if we can route session name to channel successfully...
				channel, ok := environ.SessionChannelRouter.Route(name)
				if ok {
					session.Stream.OnOrderUpdate(func(order types.Order) {
						text := util.Render(TemplateOrderReport, order)
						environ.NotifyTo(channel, text, &order)
					})
				} else {
					session.Stream.OnOrderUpdate(defaultOrderUpdateHandler)
				}
			}

		case "$symbol":
			// add object route
			environ.ObjectChannelRouter.Route(func(obj interface{}) (channel string, ok bool) {
				order, matched := obj.(*types.Order)
				if !matched {
					return
				}
				channel, ok = environ.SymbolChannelRouter.Route(order.Symbol)
				return
			})

			// use same handler for each session
			handler := func(order types.Order) {
				text := util.Render(TemplateOrderReport, order)
				channel, ok := environ.RouteObject(&order)
				if ok {
					environ.NotifyTo(channel, text, &order)
				} else {
					environ.Notify(text, &order)
				}
			}
			for _, session := range environ.sessions {
				session.Stream.OnOrderUpdate(handler)
			}
		}

		switch conf.Routing.SubmitOrder {

		case "$silent": // silent, do not setup notification

		case "$symbol":
			// add object route
			environ.ObjectChannelRouter.Route(func(obj interface{}) (channel string, ok bool) {
				order, matched := obj.(*types.SubmitOrder)
				if !matched {
					return
				}

				channel, ok = environ.SymbolChannelRouter.Route(order.Symbol)
				return
			})

		}

		// currently not used
		switch conf.Routing.PnL {
		case "$symbol":
			environ.ObjectChannelRouter.Route(func(obj interface{}) (channel string, ok bool) {
				report, matched := obj.(*pnl.AverageCostPnlReport)
				if !matched {
					return
				}
				channel, ok = environ.SymbolChannelRouter.Route(report.Symbol)
				return
			})
		}

	}
	return nil
}

func (environ *Environment) SetStartTime(t time.Time) *Environment {
	environ.startTime = t
	return environ
}

// SyncTradesFrom overrides the default trade scan time (-7 days)
func (environ *Environment) SyncTradesFrom(t time.Time) *Environment {
	environ.tradeScanTime = t
	return environ
}

func (environ *Environment) Connect(ctx context.Context) error {
	for n := range environ.sessions {
		// avoid using the placeholder variable for the session because we use that in the callbacks
		var session = environ.sessions[n]
		var logger = log.WithField("session", n)

		if len(session.Subscriptions) == 0 {
			logger.Warnf("exchange session %s has no subscriptions", session.Name)
		} else {
			// add the subscribe requests to the stream
			for _, s := range session.Subscriptions {
				logger.Infof("subscribing %s %s %v", s.Symbol, s.Channel, s.Options)
				session.Stream.Subscribe(s.Channel, s.Symbol, s.Options)
			}
		}

		logger.Infof("connecting session %s...", session.Name)
		if err := session.Stream.Connect(ctx); err != nil {
			return err
		}
	}

	return nil
}

func LoadExchangeMarketsWithCache(ctx context.Context, ex types.Exchange) (markets types.MarketMap, err error) {
	err = WithCache(fmt.Sprintf("%s-markets", ex.Name()), &markets, func() (interface{}, error) {
		return ex.QueryMarkets(ctx)
	})
	return markets, err
}

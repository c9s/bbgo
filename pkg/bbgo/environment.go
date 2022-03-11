package bbgo

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io/ioutil"
	stdlog "log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/codingconcepts/env"
	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/spf13/viper"
	"gopkg.in/tucnak/telebot.v2"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/notifier/slacknotifier"
	"github.com/c9s/bbgo/pkg/notifier/telegramnotifier"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/slack/slacklog"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func init() {
	// randomize pulling
	rand.Seed(time.Now().UnixNano())
}

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

type SyncStatus int

const (
	SyncNotStarted SyncStatus = iota
	Syncing
	SyncDone
)

// Environment presents the real exchange data layer
type Environment struct {
	// Notifiability here for environment is for the streaming data notification
	// note that, for back tests, we don't need notification.
	Notifiability

	PersistenceServiceFacade *service.PersistenceServiceFacade
	DatabaseService          *service.DatabaseService
	OrderService             *service.OrderService
	TradeService             *service.TradeService
	ProfitService            *service.ProfitService
	PositionService          *service.PositionService
	BacktestService          *service.BacktestService
	RewardService            *service.RewardService
	SyncService              *service.SyncService
	AccountService           *service.AccountService

	// startTime is the time of start point (which is used in the backtest)
	startTime time.Time

	// syncStartTime is the time point we want to start the sync (for trades and orders)
	syncStartTime time.Time
	syncMutex     sync.Mutex

	syncStatusMutex sync.Mutex
	syncStatus      SyncStatus

	sessions map[string]*ExchangeSession
}

func NewEnvironment() *Environment {
	return &Environment{
		// default trade scan time
		syncStartTime: time.Now().AddDate(-1, 0, 0), // defaults to sync from 1 year ago
		sessions:      make(map[string]*ExchangeSession),
		startTime:     time.Now(),

		syncStatus: SyncNotStarted,
		PersistenceServiceFacade: &service.PersistenceServiceFacade{
			Memory: service.NewMemoryService(),
		},
	}
}

func (environ *Environment) Session(name string) (*ExchangeSession, bool) {
	s, ok := environ.sessions[name]
	return s, ok
}

func (environ *Environment) Sessions() map[string]*ExchangeSession {
	return environ.sessions
}

func (environ *Environment) SelectSessions(names ...string) map[string]*ExchangeSession {
	if len(names) == 0 {
		return environ.sessions
	}

	sessions := make(map[string]*ExchangeSession)
	for _, name := range names {
		if s, ok := environ.Session(name); ok {
			sessions[name] = s
		}
	}

	return sessions
}

func (environ *Environment) ConfigureDatabase(ctx context.Context) error {
	// configureDB configures the database service based on the environment variable
	if driver, ok := os.LookupEnv("DB_DRIVER"); ok {

		if dsn, ok := os.LookupEnv("DB_DSN"); ok {
			return environ.ConfigureDatabaseDriver(ctx, driver, dsn)
		}

	} else if dsn, ok := os.LookupEnv("SQLITE3_DSN"); ok {

		return environ.ConfigureDatabaseDriver(ctx, "sqlite3", dsn)

	} else if dsn, ok := os.LookupEnv("MYSQL_URL"); ok {

		return environ.ConfigureDatabaseDriver(ctx, "mysql", dsn)

	}

	return nil
}

func (environ *Environment) ConfigureDatabaseDriver(ctx context.Context, driver string, dsn string) error {
	environ.DatabaseService = service.NewDatabaseService(driver, dsn)
	err := environ.DatabaseService.Connect()
	if err != nil {
		return err
	}

	if err := environ.DatabaseService.Upgrade(ctx); err != nil {
		return err
	}

	// get the db connection pool object to create other services
	db := environ.DatabaseService.DB
	environ.OrderService = &service.OrderService{DB: db}
	environ.TradeService = &service.TradeService{DB: db}
	environ.RewardService = &service.RewardService{DB: db}
	environ.AccountService = &service.AccountService{DB: db}
	environ.ProfitService = &service.ProfitService{DB: db}
	environ.PositionService = &service.PositionService{DB: db}

	environ.SyncService = &service.SyncService{
		TradeService:    environ.TradeService,
		OrderService:    environ.OrderService,
		RewardService:   environ.RewardService,
		WithdrawService: &service.WithdrawService{DB: db},
		DepositService:  &service.DepositService{DB: db},
	}

	return nil
}

// AddExchangeSession adds the existing exchange session or pre-created exchange session
func (environ *Environment) AddExchangeSession(name string, session *ExchangeSession) *ExchangeSession {
	// update Notifiability from the environment
	session.Notifiability = environ.Notifiability

	environ.sessions[name] = session
	return session
}

// AddExchange adds the given exchange with the session name, this is the default
func (environ *Environment) AddExchange(name string, exchange types.Exchange) (session *ExchangeSession) {
	session = NewExchangeSession(name, exchange)
	return environ.AddExchangeSession(name, session)
}

func (environ *Environment) ConfigureExchangeSessions(userConfig *Config) error {
	// if sessions are not defined, we detect the sessions automatically
	if len(userConfig.Sessions) == 0 {
		return environ.AddExchangesByViperKeys()
	}

	return environ.AddExchangesFromSessionConfig(userConfig.Sessions)
}

func (environ *Environment) AddExchangesByViperKeys() error {
	for _, n := range types.SupportedExchanges {
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

func (environ *Environment) AddExchangesFromSessionConfig(sessions map[string]*ExchangeSession) error {
	for sessionName, session := range sessions {
		if err := session.InitExchange(sessionName, nil); err != nil {
			return err
		}

		environ.AddExchangeSession(sessionName, session)
	}

	return nil
}

// Init prepares the data that will be used by the strategies
func (environ *Environment) Init(ctx context.Context) (err error) {
	for n := range environ.sessions {
		var session = environ.sessions[n]
		if err = session.Init(ctx, environ); err != nil {
			// we can skip initialized sessions
			if err != ErrSessionAlreadyInitialized {
				return err
			}
		}
	}

	return
}

// Start initializes the symbols data streams
func (environ *Environment) Start(ctx context.Context) (err error) {
	for n := range environ.sessions {
		var session = environ.sessions[n]
		if err = session.InitSymbols(ctx, environ); err != nil {
			return err
		}
	}
	return
}

func (environ *Environment) ConfigurePersistence(conf *PersistenceConfig) error {
	if conf.Redis != nil {
		if err := env.Set(conf.Redis); err != nil {
			return err
		}

		environ.PersistenceServiceFacade.Redis = service.NewRedisPersistenceService(conf.Redis)
	}

	if conf.Json != nil {
		if _, err := os.Stat(conf.Json.Directory); os.IsNotExist(err) {
			if err2 := os.MkdirAll(conf.Json.Directory, 0777); err2 != nil {
				log.WithError(err2).Errorf("can not create directory: %s", conf.Json.Directory)
				return err2
			}
		}

		environ.PersistenceServiceFacade.Json = &service.JsonPersistenceService{Directory: conf.Json.Directory}
	}

	return nil
}

// ConfigureNotificationRouting configures the notification rules
// for symbol-based routes, we should register the same symbol rules for each session.
// for session-based routes, we should set the fixed callbacks for each session
func (environ *Environment) ConfigureNotificationRouting(conf *NotificationConfig) error {
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
				environ.Notify(&trade)
			}
			for name := range environ.sessions {
				session := environ.sessions[name]

				// if we can route session name to channel successfully...
				channel, ok := environ.SessionChannelRouter.Route(name)
				if ok {
					session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
						environ.NotifyTo(channel, &trade)
					})
				} else {
					session.UserDataStream.OnTradeUpdate(defaultTradeUpdateHandler)
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
				channel, ok := environ.RouteObject(&trade)
				if ok {
					environ.NotifyTo(channel, &trade)
				} else {
					environ.Notify(&trade)
				}
			}
			for _, session := range environ.sessions {
				session.UserDataStream.OnTradeUpdate(handler)
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
					session.UserDataStream.OnOrderUpdate(func(order types.Order) {
						text := util.Render(TemplateOrderReport, order)
						environ.NotifyTo(channel, text, &order)
					})
				} else {
					session.UserDataStream.OnOrderUpdate(defaultOrderUpdateHandler)
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
				session.UserDataStream.OnOrderUpdate(handler)
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

		// currently, not used
		// FIXME: this is causing cyclic import
		/*
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
		*/

	}
	return nil
}

func (environ *Environment) SetStartTime(t time.Time) *Environment {
	environ.startTime = t
	return environ
}

// SetSyncStartTime overrides the default trade scan time (-7 days)
func (environ *Environment) SetSyncStartTime(t time.Time) *Environment {
	environ.syncStartTime = t
	return environ
}

func (environ *Environment) BindSync(userConfig *Config) {
	if environ.BacktestService != nil {
		return
	}

	// If trade service is configured, we have the db configured
	if environ.TradeService == nil {
		return
	}

	if userConfig.Sync == nil || userConfig.Sync.UserDataStream == nil {
		return
	}

	tradeWriter := func(trade types.Trade) {
		if err := environ.TradeService.Insert(trade); err != nil {
			log.WithError(err).Errorf("trade insert error: %+v", trade)
		}
	}
	orderWriter := func(order types.Order) {
		switch order.Status {
		case types.OrderStatusFilled, types.OrderStatusCanceled:
			if order.ExecutedQuantity.Sign() > 0 {
				if err := environ.OrderService.Insert(order); err != nil {
					log.WithError(err).Errorf("order insert error: %+v", order)
				}
			}
		}
	}

	for _, session := range environ.sessions {
		if userConfig.Sync.UserDataStream.Trades {
			session.UserDataStream.OnTradeUpdate(tradeWriter)
		}
		if userConfig.Sync.UserDataStream.FilledOrders {
			session.UserDataStream.OnOrderUpdate(orderWriter)
		}
	}
}

func (environ *Environment) Connect(ctx context.Context) error {
	log.Debugf("starting interaction...")
	if err := interact.Start(ctx); err != nil {
		return err
	}

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
				session.MarketDataStream.Subscribe(s.Channel, s.Symbol, s.Options)
			}
		}

		logger.Infof("connecting %s market data stream...", session.Name)
		if err := session.MarketDataStream.Connect(ctx); err != nil {
			return err
		}

		if !session.PublicOnly {
			logger.Infof("connecting %s user data stream...", session.Name)
			if err := session.UserDataStream.Connect(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (environ *Environment) IsSyncing() (status SyncStatus) {
	environ.syncStatusMutex.Lock()
	status = environ.syncStatus
	environ.syncStatusMutex.Unlock()
	return status
}

func (environ *Environment) setSyncing(status SyncStatus) {
	environ.syncStatusMutex.Lock()
	environ.syncStatus = status
	environ.syncStatusMutex.Unlock()
}

// Sync syncs all registered exchange sessions
func (environ *Environment) Sync(ctx context.Context, userConfig ...*Config) error {
	if environ.SyncService == nil {
		return nil
	}

	environ.syncMutex.Lock()
	defer environ.syncMutex.Unlock()

	environ.setSyncing(Syncing)
	defer environ.setSyncing(SyncDone)

	// sync by the defined user config
	if len(userConfig) > 0 && userConfig[0] != nil && userConfig[0].Sync != nil {
		syncSymbols := userConfig[0].Sync.Symbols
		sessions := environ.sessions
		selectedSessions := userConfig[0].Sync.Sessions
		if len(selectedSessions) > 0 {
			sessions = environ.SelectSessions(selectedSessions...)
		}
		for _, session := range sessions {
			if err := environ.syncSession(ctx, session, syncSymbols...); err != nil {
				return err
			}
		}
		return nil
	}

	// the default sync logics
	for _, session := range environ.sessions {
		if err := environ.syncSession(ctx, session); err != nil {
			return err
		}
	}

	return nil
}

func (environ *Environment) RecordPosition(position *types.Position, trade types.Trade, profit types.Profit) {
	// skip for back-test
	if environ.BacktestService != nil {
		return
	}

	if environ.DatabaseService == nil {
		return
	}

	if environ.ProfitService == nil {
		return
	}

	if position.Strategy == "" && profit.Strategy != "" {
		position.Strategy = profit.Strategy
	}

	if position.StrategyInstanceID == "" && profit.StrategyInstanceID != "" {
		position.StrategyInstanceID = profit.StrategyInstanceID
	}

	if err := environ.PositionService.Insert(position, trade, profit.Profit); err != nil {
		log.WithError(err).Errorf("can not insert position record")
	}

	if err := environ.ProfitService.Insert(profit); err != nil {
		log.WithError(err).Errorf("can not insert profit record: %+v", profit)
	}
}

func (environ *Environment) RecordProfit(profit types.Profit) {
	// skip for back-test
	if environ.BacktestService != nil {
		return
	}

	if environ.DatabaseService == nil {
		return
	}
	if environ.ProfitService == nil {
		return
	}

	if err := environ.ProfitService.Insert(profit); err != nil {
		log.WithError(err).Errorf("can not insert profit record: %+v", profit)
	}
}

func (environ *Environment) SyncSession(ctx context.Context, session *ExchangeSession, defaultSymbols ...string) error {
	if environ.SyncService == nil {
		return nil
	}

	environ.syncMutex.Lock()
	defer environ.syncMutex.Unlock()

	environ.setSyncing(Syncing)
	defer environ.setSyncing(SyncDone)

	return environ.syncSession(ctx, session, defaultSymbols...)
}

func (environ *Environment) syncSession(ctx context.Context, session *ExchangeSession, defaultSymbols ...string) error {
	symbols, err := session.getSessionSymbols(defaultSymbols...)
	if err != nil {
		return err
	}

	log.Infof("syncing symbols %v from session %s", symbols, session.Name)

	return environ.SyncService.SyncSessionSymbols(ctx, session.Exchange, environ.syncStartTime, symbols...)
}

func (environ *Environment) ConfigureNotificationSystem(userConfig *Config) error {
	environ.Notifiability = Notifiability{
		SymbolChannelRouter:  NewPatternChannelRouter(nil),
		SessionChannelRouter: NewPatternChannelRouter(nil),
		ObjectChannelRouter:  NewObjectChannelRouter(),
	}

	// setup default notification config
	if userConfig.Notifications == nil {
		userConfig.Notifications = &NotificationConfig{
			Routing: &SlackNotificationRouting{
				Trade:       "$session",
				Order:       "$silent",
				SubmitOrder: "$silent",
			},
		}
	}

	var persistence = environ.PersistenceServiceFacade.Get()

	err := environ.setupInteraction(persistence)
	if err != nil {
		return err
	}

	// setup slack
	slackToken := viper.GetString("slack-token")
	if len(slackToken) > 0 && userConfig.Notifications != nil {
		environ.setupSlack(userConfig, slackToken, persistence)
	}

	// check if telegram bot token is defined
	telegramBotToken := viper.GetString("telegram-bot-token")
	if len(telegramBotToken) > 0 {
		if err := environ.setupTelegram(userConfig, telegramBotToken, persistence); err != nil {
			return err
		}
	}

	if userConfig.Notifications != nil {
		if err := environ.ConfigureNotificationRouting(userConfig.Notifications); err != nil {
			return err
		}
	}

	return nil
}

// getAuthStoreID returns the authentication store id
// if telegram bot token is defined, the bot id will be used.
// if not, env var $USER will be used.
// if both are not defined, a default "default" will be used.
func getAuthStoreID() string {
	telegramBotToken := viper.GetString("telegram-bot-token")
	if len(telegramBotToken) > 0 {
		tt := strings.Split(telegramBotToken, ":")
		return tt[0]
	}

	userEnv := os.Getenv("USER")
	if userEnv != "" {
		return userEnv
	}

	return "default"
}

func (environ *Environment) setupInteraction(persistence service.PersistenceService) error {
	var otpQRCodeImagePath = fmt.Sprintf("otp.png")
	var key *otp.Key
	var keySecret string
	var authStore = environ.getAuthStore(persistence)
	if err := authStore.Load(&keySecret); err != nil {
		log.Warnf("telegram session not found, generating new one-time password key for new telegram session...")

		newKey, err := setupNewOTPKey(otpQRCodeImagePath)
		if err != nil {
			return errors.Wrapf(err, "failed to setup totp (time-based one time password) key")
		}

		key = newKey
		keySecret = key.Secret()
		if err := authStore.Save(keySecret); err != nil {
			return err
		}

		printOtpAuthGuide(otpQRCodeImagePath)

	} else if keySecret != "" {
		key, err = otp.NewKeyFromURL(keySecret)
		if err != nil {
			return err
		}

		log.Infof("otp key loaded: %s", util.MaskKey(key.Secret()))
		printOtpAuthGuide(otpQRCodeImagePath)
	}

	authStrict := false
	authMode := interact.AuthModeToken
	authToken := viper.GetString("telegram-bot-auth-token")

	if authToken != "" && key != nil {
		authStrict = true
	} else if authToken != "" {
		authMode = interact.AuthModeToken
	} else if key != nil {
		authMode = interact.AuthModeOTP
	}

	if authMode == interact.AuthModeToken {
		log.Debugf("found interaction auth token, using token mode for authorization...")
		printAuthTokenGuide(authToken)
	}

	interact.AddCustomInteraction(&interact.AuthInteract{
		Strict:             authStrict,
		Mode:               authMode,
		Token:              authToken, // can be empty string here
		OneTimePasswordKey: key,       // can be nil here
	})
	return nil
}

func (environ *Environment) getAuthStore(persistence service.PersistenceService) service.Store {
	id := getAuthStoreID()
	return persistence.NewStore("bbgo", "auth", id)
}

func (environ *Environment) setupSlack(userConfig *Config, slackToken string, persistence service.PersistenceService) {
	conf := userConfig.Notifications.Slack
	if conf == nil {
		return
	}

	if !strings.HasPrefix(slackToken, "xoxb-") {
		log.Error("SLACK_BOT_TOKEN must have the prefix \"xoxb-\".")
		return
	}

	// app-level token (for specific api)
	slackAppToken := viper.GetString("slack-app-token")
	if !strings.HasPrefix(slackAppToken, "xapp-") {
		log.Errorf("SLACK_APP_TOKEN must have the prefix \"xapp-\".")
		return
	}

	if conf.ErrorChannel != "" {
		log.Debugf("found slack configured, setting up log hook...")
		log.AddHook(slacklog.NewLogHook(slackToken, conf.ErrorChannel))
	}

	log.Debugf("adding slack notifier with default channel: %s", conf.DefaultChannel)

	var client = slack.New(slackToken,
		slack.OptionDebug(true),
		slack.OptionLog(stdlog.New(os.Stdout, "api: ", stdlog.Lshortfile|stdlog.LstdFlags)),
		slack.OptionAppLevelToken(slackAppToken))

	var notifier = slacknotifier.New(client, conf.DefaultChannel)
	environ.AddNotifier(notifier)

	// allocate a store, so that we can save the chatID for the owner
	var messenger = interact.NewSlack(client)

	var sessions = interact.SlackSessionMap{}
	var sessionStore = persistence.NewStore("bbgo", "slack")
	if err := sessionStore.Load(&sessions); err != nil {

	} else {
		// TODO: this is not necessary for slack, but we should find a way to restore the sessions
		/*
			for _, session := range sessions {
				if session.IsAuthorized() {
					// notifier.AddChat(session.Chat)
				}
			}
			messenger.RestoreSessions(sessions)
			messenger.OnAuthorized(func(userSession *interact.SlackSession) {
				if userSession.IsAuthorized() {
					// notifier.AddChat(userSession.Chat)
				}
			})
		*/
	}

	interact.AddMessenger(messenger)
}

func (environ *Environment) setupTelegram(userConfig *Config, telegramBotToken string, persistence service.PersistenceService) error {
	tt := strings.Split(telegramBotToken, ":")
	telegramID := tt[0]

	bot, err := telebot.NewBot(telebot.Settings{
		// You can also set custom API URL.
		// If field is empty it equals to "https://api.telegram.org".
		// URL: "http://195.129.111.17:8012",
		Token:  telegramBotToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		return err
	}

	var opts []telegramnotifier.Option
	if userConfig.Notifications != nil && userConfig.Notifications.Telegram != nil {
		log.Infof("telegram broadcast is enabled")
		opts = append(opts, telegramnotifier.UseBroadcast())
	}

	var notifier = telegramnotifier.New(bot, opts...)
	environ.Notifiability.AddNotifier(notifier)

	// allocate a store, so that we can save the chatID for the owner
	var messenger = interact.NewTelegram(bot)

	var sessions = interact.TelegramSessionMap{}
	var sessionStore = persistence.NewStore("bbgo", "telegram", telegramID)
	if err := sessionStore.Load(&sessions); err != nil {
		if err != service.ErrPersistenceNotExists {
			log.WithError(err).Errorf("unexpected persistence error")
		}
	} else {
		for _, session := range sessions {
			if session.IsAuthorized() {
				notifier.AddChat(session.Chat)
			}
		}

		// you must restore the session after the notifier updates
		messenger.RestoreSessions(sessions)
	}

	messenger.OnAuthorized(func(userSession *interact.TelegramSession) {
		if userSession.IsAuthorized() {
			notifier.AddChat(userSession.Chat)
		}

		log.Infof("user session %d got authorized, saving telegram sessions...", userSession.User.ID)
		if err := sessionStore.Save(messenger.Sessions()); err != nil {
			log.WithError(err).Errorf("telegram session save error")
		}
	})

	interact.AddMessenger(messenger)
	return nil
}

func writeOTPKeyAsQRCodePNG(key *otp.Key, imagePath string) error {
	// Convert TOTP key into a PNG
	var buf bytes.Buffer
	img, err := key.Image(512, 512)
	if err != nil {
		return err
	}

	if err := png.Encode(&buf, img); err != nil {
		return err
	}

	if err := ioutil.WriteFile(imagePath, buf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

// setupNewOTPKey generates a new otp key and save the secret as a qrcode image
func setupNewOTPKey(qrcodeImagePath string) (*otp.Key, error) {
	key, err := service.NewDefaultTotpKey()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to setup totp (time-based one time password) key")
	}

	printOtpKey(key)

	if err := writeOTPKeyAsQRCodePNG(key, qrcodeImagePath); err != nil {
		return nil, err
	}

	return key, nil
}

func printOtpKey(key *otp.Key) {
	fmt.Println("")
	fmt.Println("====================================================================")
	fmt.Println("               PLEASE STORE YOUR OTP KEY SAFELY                     ")
	fmt.Println("====================================================================")
	fmt.Printf("  Issuer:       %s\n", key.Issuer())
	fmt.Printf("  AccountName:  %s\n", key.AccountName())
	fmt.Printf("  Secret:       %s\n", key.Secret())
	fmt.Printf("  Key URL:      %s\n", key.URL())
	fmt.Println("====================================================================")
	fmt.Println("")
}

func printOtpAuthGuide(qrcodeImagePath string) {
	fmt.Printf(`
To scan your OTP QR code, please run the following command:
	
	open %s

For telegram, send the auth command with the generated one-time password to the bbo bot you created to enable the notification:

	/auth

`, qrcodeImagePath)
}

func printAuthTokenGuide(token string) {
	fmt.Printf(`
For telegram, send the following command to the bbgo bot you created to enable the notification:

	/auth

And then enter your token

	%s

`, token)
}

func (session *ExchangeSession) getSessionSymbols(defaultSymbols ...string) ([]string, error) {
	if session.IsolatedMargin {
		return []string{session.IsolatedMarginSymbol}, nil
	}

	if len(defaultSymbols) > 0 {
		return defaultSymbols, nil
	}

	return session.FindPossibleSymbols()
}

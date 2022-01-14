package bbgo

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/codingconcepts/env"
	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	log "github.com/sirupsen/logrus"
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
	BacktestService          *service.BacktestService
	RewardService            *service.RewardService
	SyncService              *service.SyncService
	AccountService 			 *service.AccountService

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
func (environ *Environment) Sync(ctx context.Context) error {
	if environ.SyncService == nil {
		return nil
	}

	environ.syncMutex.Lock()
	defer environ.syncMutex.Unlock()

	environ.setSyncing(Syncing)
	defer environ.setSyncing(SyncDone)

	for _, session := range environ.sessions {
		if err := environ.syncSession(ctx, session); err != nil {
			return err
		}
	}

	return nil
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
	symbols, err := getSessionSymbols(session, defaultSymbols...)
	if err != nil {
		return err
	}

	log.Infof("syncing symbols %v from session %s", symbols, session.Name)

	return environ.SyncService.SyncSessionSymbols(ctx, session.Exchange, environ.syncStartTime, symbols...)
}

func getSessionSymbols(session *ExchangeSession, defaultSymbols ...string) ([]string, error) {
	if session.IsolatedMargin {
		return []string{session.IsolatedMarginSymbol}, nil
	}

	if len(defaultSymbols) > 0 {
		return defaultSymbols, nil
	}

	return session.FindPossibleSymbols()
}

func (environ *Environment) ConfigureNotificationSystem(userConfig *Config) error {
	environ.Notifiability = Notifiability{
		SymbolChannelRouter:  NewPatternChannelRouter(nil),
		SessionChannelRouter: NewPatternChannelRouter(nil),
		ObjectChannelRouter:  NewObjectChannelRouter(),
	}

	slackToken := viper.GetString("slack-token")
	if len(slackToken) > 0 && userConfig.Notifications != nil {
		if conf := userConfig.Notifications.Slack; conf != nil {
			if conf.ErrorChannel != "" {
				log.Debugf("found slack configured, setting up log hook...")
				log.AddHook(slacklog.NewLogHook(slackToken, conf.ErrorChannel))
			}

			log.Debugf("adding slack notifier with default channel: %s", conf.DefaultChannel)
			var notifier = slacknotifier.New(slackToken, conf.DefaultChannel)
			environ.AddNotifier(notifier)
		}
	}

	persistence := environ.PersistenceServiceFacade.Get()
	telegramBotToken := viper.GetString("telegram-bot-token")
	if len(telegramBotToken) > 0 {
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

		// allocate a store, so that we can save the chatID for the owner
		var sessionStore = persistence.NewStore("bbgo", "telegram", telegramID)

		interact.SetMessenger(&interact.Telegram{
			Private: true,
			Bot:     bot,
		})


		// TODO: replace this background context later
		if err := interact.Start(context.Background()); err != nil {
			return err
		}

		authToken := viper.GetString("telegram-bot-auth-token")
		if len(authToken) > 0 {
			interact.AddCustomInteraction(&interact.AuthInteract{
				Strict: true,
				Mode:   interact.AuthModeToken,
				Token:  authToken,
			})

			log.Debugf("telegram bot auth token is set, using fixed token for authorization...")
			printTelegramAuthTokenGuide(authToken)
		}

		var session telegramnotifier.Session
		var qrcodeImagePath = fmt.Sprintf("otp-%s.png", telegramID)
		if err := sessionStore.Load(&session); err != nil || session.Owner == nil {
			log.Warnf("telegram session not found, generating new one-time password key for new telegram session...")

			key, err := setupNewOTPKey(qrcodeImagePath)
			if err != nil {
				return errors.Wrapf(err, "failed to setup totp (time-based one time password) key")
			}

			printTelegramOtpAuthGuide(qrcodeImagePath)

			session = telegramnotifier.NewSession(key)
			if err := sessionStore.Save(&session); err != nil {
				return errors.Wrap(err, "failed to save session")
			}
		} else if session.OneTimePasswordKey != nil {
			log.Infof("telegram session loaded: %+v", session)

			printTelegramOtpAuthGuide(qrcodeImagePath)
		}

		go interaction.Start(session)

		var opts []telegramnotifier.Option

		if userConfig.Notifications != nil && userConfig.Notifications.Telegram != nil {
			log.Infof("telegram broadcast is enabled")
			opts = append(opts, telegramnotifier.UseBroadcast())
		}

		var notifier = telegramnotifier.New(opts...)
		environ.Notifiability.AddNotifier(notifier)
	}

	if userConfig.Notifications == nil {
		userConfig.Notifications = &NotificationConfig{
			Routing :  &SlackNotificationRouting{
				Trade: "$session",
				Order: "$session",
			},
		}
	}

	if userConfig.Notifications != nil {
		if err := environ.ConfigureNotificationRouting(userConfig.Notifications); err != nil {
			return err
		}
	}

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
	fmt.Println("====================PLEASE STORE YOUR OTP KEY=======================")
	fmt.Println("")
	fmt.Printf("Issuer:       %s\n", key.Issuer())
	fmt.Printf("AccountName:  %s\n", key.AccountName())
	fmt.Printf("Secret:       %s\n", key.Secret())
	fmt.Printf("Key URL:      %s\n", key.URL())
	fmt.Println("")
	fmt.Println("====================================================================")
	fmt.Println("")
}

func printTelegramOtpAuthGuide(qrcodeImagePath string) {
	fmt.Printf(`
To scan your OTP QR code, please run the following command:
	
	open %s

send the auth command with the generated one-time password to the bbo bot you created to enable the notification:

	/auth {code}

`, qrcodeImagePath)
}

func printTelegramAuthTokenGuide(token string) {
	fmt.Printf(`
send the following command to the bbgo bot you created to enable the notification:

	/auth %s

`, token)
}

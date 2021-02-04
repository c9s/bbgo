package cmd

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/notifier/slacknotifier"
	"github.com/c9s/bbgo/pkg/notifier/telegramnotifier"
	"github.com/c9s/bbgo/pkg/server"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/slack/slacklog"
)

func init() {
	RunCmd.Flags().Bool("no-compile", false, "do not compile wrapper binary")

	RunCmd.Flags().String("totp-key-url", "", "time-based one-time password key URL, if defined, it will be used for restoring the otp key")
	RunCmd.Flags().String("totp-issuer", "", "")
	RunCmd.Flags().String("totp-account-name", "", "")
	RunCmd.Flags().Bool("enable-web-server", false, "enable web server")
	RunCmd.Flags().Bool("setup", false, "use setup mode")

	RunCmd.Flags().Bool("no-dotenv", false, "disable built-in dotenv")
	RunCmd.Flags().String("dotenv", ".env.local", "the dotenv file you want to load")

	RunCmd.Flags().String("since", "", "pnl since time")
	RootCmd.AddCommand(RunCmd)
}

var RunCmd = &cobra.Command{
	Use:   "run",
	Short: "run strategies from config file",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE:         run,
}

func runSetup(baseCtx context.Context, userConfig *bbgo.Config, enableApiServer bool) error {
	ctx, cancelTrading := context.WithCancel(baseCtx)
	defer cancelTrading()

	environ := bbgo.NewEnvironment()

	trader := bbgo.NewTrader(environ)

	if enableApiServer {
		go func() {
			s := &server.Server{
				Config:  userConfig,
				Environ: environ,
				Trader:  trader,
				OpenInBrowser: true,
				Setup: &server.Setup{
					Context: ctx,
					Cancel:  cancelTrading,
					Token:   "",
				},
			}

			if err := s.Run(ctx); err != nil {
				log.WithError(err).Errorf("server error")
			}
		}()
	}

	cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
	cancelTrading()

	shutdownCtx, cancelShutdown := context.WithDeadline(ctx, time.Now().Add(30*time.Second))

	log.Infof("shutting down...")
	trader.Graceful.Shutdown(shutdownCtx)
	cancelShutdown()
	return nil
}

func runConfig(basectx context.Context, userConfig *bbgo.Config, enableApiServer bool) error {
	ctx, cancelTrading := context.WithCancel(basectx)
	defer cancelTrading()

	environ := bbgo.NewEnvironment()

	if viper.IsSet("mysql-url") {
		dsn := viper.GetString("mysql-url")
		if err := environ.ConfigureDatabase(ctx, dsn); err != nil {
			return err
		}
	}

	if err := environ.AddExchangesFromConfig(userConfig); err != nil {
		return err
	}

	if userConfig.Persistence != nil {
		if err := environ.ConfigurePersistence(userConfig.Persistence); err != nil {
			return err
		}
	}

	notification := bbgo.Notifiability{
		SymbolChannelRouter:  bbgo.NewPatternChannelRouter(nil),
		SessionChannelRouter: bbgo.NewPatternChannelRouter(nil),
		ObjectChannelRouter:  bbgo.NewObjectChannelRouter(),
	}

	// for slack
	slackToken := viper.GetString("slack-token")
	if len(slackToken) > 0 && userConfig.Notifications != nil {
		if conf := userConfig.Notifications.Slack; conf != nil {
			if conf.ErrorChannel != "" {
				log.Infof("found slack configured, setting up log hook...")
				log.AddHook(slacklog.NewLogHook(slackToken, conf.ErrorChannel))
			}

			log.Infof("adding slack notifier with default channel: %s", conf.DefaultChannel)
			var notifier = slacknotifier.New(slackToken, conf.DefaultChannel)
			notification.AddNotifier(notifier)
		}
	}

	// for telegram
	telegramBotToken := viper.GetString("telegram-bot-token")
	telegramBotAuthToken := viper.GetString("telegram-bot-auth-token")
	if len(telegramBotToken) > 0 {
		log.Infof("initializing telegram bot...")

		bot, err := tb.NewBot(tb.Settings{
			// You can also set custom API URL.
			// If field is empty it equals to "https://api.telegram.org".
			// URL: "http://195.129.111.17:8012",
			Token:  telegramBotToken,
			Poller: &tb.LongPoller{Timeout: 10 * time.Second},
		})

		if err != nil {
			return err
		}

		var persistence bbgo.PersistenceService = bbgo.NewMemoryService()
		var sessionStore = persistence.NewStore("bbgo", "telegram")

		tt := strings.Split(bot.Token, ":")
		telegramID := tt[0]

		if environ.PersistenceServiceFacade != nil {
			if environ.PersistenceServiceFacade.Redis != nil {
				persistence = environ.PersistenceServiceFacade.Redis
				sessionStore = persistence.NewStore("bbgo", "telegram", telegramID)
			}
		}

		interaction := telegramnotifier.NewInteraction(bot, sessionStore)

		if len(telegramBotAuthToken) > 0 {
			log.Infof("telegram bot auth token is set, using fixed token for authorization...")
			interaction.SetAuthToken(telegramBotAuthToken)
			log.Infof("send the following command to the bbgo bot you created to enable the notification")
			log.Infof("")
			log.Infof("")
			log.Infof("    /auth %s", telegramBotAuthToken)
			log.Infof("")
			log.Infof("")
		}

		var session telegramnotifier.Session
		if err := sessionStore.Load(&session); err != nil || session.Owner == nil {
			log.Warnf("session not found, generating new one-time password key for new session...")

			key, err := service.NewDefaultTotpKey()
			if err != nil {
				return errors.Wrapf(err, "failed to setup totp (time-based one time password) key")
			}

			displayOTPKey(key)

			qrcodeImagePath := fmt.Sprintf("otp-%s.png", telegramID)

			err = writeOTPKeyAsQRCodePNG(key, qrcodeImagePath)
			log.Infof("To scan your OTP QR code, please run the following command:")
			log.Infof("")
			log.Infof("")
			log.Infof("     open %s", qrcodeImagePath)
			log.Infof("")
			log.Infof("")
			log.Infof("send the auth command with the generated one-time password to the bbgo bot you created to enable the notification")
			log.Infof("")
			log.Infof("")
			log.Infof("     /auth {code}")
			log.Infof("")
			log.Infof("")

			session = telegramnotifier.NewSession(key)
			if err := sessionStore.Save(&session); err != nil {
				return errors.Wrap(err, "failed to save session")
			}
		}

		go interaction.Start(session)

		var notifier = telegramnotifier.New(interaction)
		notification.AddNotifier(notifier)
	}

	environ.Notifiability = notification

	if userConfig.Notifications != nil {
		if err := environ.ConfigureNotification(userConfig.Notifications); err != nil {
			return err
		}
	}

	trader := bbgo.NewTrader(environ)

	if userConfig.RiskControls != nil {
		trader.SetRiskControls(userConfig.RiskControls)
	}

	for _, entry := range userConfig.ExchangeStrategies {
		for _, mount := range entry.Mounts {
			log.Infof("attaching strategy %T on %s...", entry.Strategy, mount)
			trader.AttachStrategyOn(mount, entry.Strategy)
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

	trader.Subscribe()

	if err := trader.Run(ctx); err != nil {
		return err
	}

	if enableApiServer {
		go func() {
			s := &server.Server{
				Config:  userConfig,
				Environ: environ,
				Trader:  trader,
			}

			if err := s.Run(ctx); err != nil {
				log.WithError(err).Errorf("server error")
			}
		}()
	}

	cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)

	cancelTrading()

	shutdownCtx, cancelShutdown := context.WithDeadline(ctx, time.Now().Add(30*time.Second))

	log.Infof("shutting down...")
	trader.Graceful.Shutdown(shutdownCtx)
	cancelShutdown()
	return nil
}

func run(cmd *cobra.Command, args []string) error {
	disableDotEnv, err := cmd.Flags().GetBool("no-dotenv")
	if err != nil {
		return err
	}

	if !disableDotEnv {
		dotenvFile, err := cmd.Flags().GetString("dotenv")
		if err != nil {
			return err
		}

		if err := godotenv.Load(dotenvFile); err != nil {
			return errors.Wrap(err, "error loading dotenv file")
		}
	}

	setup, err := cmd.Flags().GetBool("setup")
	if err != nil {
		return err
	}

	enableApiServer, err := cmd.Flags().GetBool("enable-web-server")
	if err != nil {
		return err
	}

	noCompile, err := cmd.Flags().GetBool("no-compile")
	if err != nil {
		return err
	}

	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return err
	}

	var userConfig *bbgo.Config

	if setup {
		log.Infof("running in setup mode, skip reading config file")
		enableApiServer = true
		userConfig = &bbgo.Config{
			Notifications:      nil,
			Persistence:        nil,
			Sessions:           nil,
			ExchangeStrategies: nil,
		}
	} else {
		if len(configFile) == 0 {
			return errors.New("--config option is required")
		}

		userConfig, err = bbgo.Load(configFile, false)
		if err != nil {
			return err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// for wrapper binary, we can just run the strategies
	if bbgo.IsWrapperBinary || (userConfig.Build != nil && len(userConfig.Build.Imports) == 0) || noCompile {
		if bbgo.IsWrapperBinary {
			log.Infof("running wrapper binary...")
		}

		if setup {
			return runSetup(ctx, userConfig, enableApiServer)
		}

		userConfig, err = bbgo.Load(configFile, true)
		if err != nil {
			return err
		}

		return runConfig(ctx, userConfig, enableApiServer)
	}

	return runWrapperBinary(ctx, userConfig, cmd, args)
}

func runWrapperBinary(ctx context.Context, userConfig *bbgo.Config, cmd *cobra.Command, args []string) error {
	var runArgs = []string{"run"}
	cmd.Flags().Visit(func(flag *flag.Flag) {
		runArgs = append(runArgs, "--"+flag.Name, flag.Value.String())
	})
	runArgs = append(runArgs, args...)

	runCmd, err := buildAndRun(ctx, userConfig, runArgs...)
	if err != nil {
		return err
	}

	if sig := cmdutil.WaitForSignal(ctx, syscall.SIGTERM, syscall.SIGINT); sig != nil {
		log.Infof("sending signal to the child process...")
		if err := runCmd.Process.Signal(sig); err != nil {
			return err
		}

		if err := runCmd.Wait(); err != nil {
			return err
		}
	}

	return nil
}

// buildAndRun builds the package natively and run the binary with the given args
func buildAndRun(ctx context.Context, userConfig *bbgo.Config, args ...string) (*exec.Cmd, error) {
	packageDir, err := ioutil.TempDir("build", "bbgow")
	if err != nil {
		return nil, err
	}

	defer os.RemoveAll(packageDir)

	targetConfig := bbgo.GetNativeBuildTargetConfig()
	binary, err := bbgo.Build(ctx, userConfig, targetConfig)
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	executePath := filepath.Join(cwd, binary)
	runCmd := exec.Command(executePath, args...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	return runCmd, runCmd.Start()
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

func displayOTPKey(key *otp.Key) {
	log.Infof("")
	log.Infof("====================PLEASE STORE YOUR OTP KEY=======================")
	log.Infof("")
	log.Infof("Issuer:       %s", key.Issuer())
	log.Infof("AccountName:  %s", key.AccountName())
	log.Infof("Secret:       %s", key.Secret())
	log.Infof("Key URL:      %s", key.URL())
	log.Infof("")
	log.Infof("====================================================================")
	log.Infof("")
}

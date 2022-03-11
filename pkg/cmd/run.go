package cmd

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/server"
)

func init() {
	RunCmd.Flags().Bool("no-compile", false, "do not compile wrapper binary")
	RunCmd.Flags().Bool("no-sync", false, "do not sync on startup")
	RunCmd.Flags().String("totp-key-url", "", "time-based one-time password key URL, if defined, it will be used for restoring the otp key")
	RunCmd.Flags().String("totp-issuer", "", "")
	RunCmd.Flags().String("totp-account-name", "", "")
	RunCmd.Flags().Bool("enable-webserver", false, "enable webserver")
	RunCmd.Flags().Bool("enable-web-server", false, "legacy option, this is renamed to --enable-webserver")
	RunCmd.Flags().String("cpu-profile", "", "cpu profile")
	RunCmd.Flags().String("webserver-bind", ":8080", "webserver binding")
	RunCmd.Flags().Bool("setup", false, "use setup mode")
	RootCmd.AddCommand(RunCmd)
}

var RunCmd = &cobra.Command{
	Use:   "run",
	Short: "run strategies from config file",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,
	RunE: run,
}

func runSetup(baseCtx context.Context, userConfig *bbgo.Config, enableApiServer bool) error {
	ctx, cancelTrading := context.WithCancel(baseCtx)
	defer cancelTrading()

	environ := bbgo.NewEnvironment()

	trader := bbgo.NewTrader(environ)

	if enableApiServer {
		go func() {
			s := &server.Server{
				Config:        userConfig,
				Environ:       environ,
				Trader:        trader,
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

	// graceful period = 15 second
	shutdownCtx, cancelShutdown := context.WithDeadline(ctx, time.Now().Add(15*time.Second))

	log.Infof("shutting down...")
	trader.Graceful.Shutdown(shutdownCtx)
	cancelShutdown()
	return nil
}

func BootstrapBacktestEnvironment(ctx context.Context, environ *bbgo.Environment, userConfig *bbgo.Config) error {
	if err := environ.ConfigureDatabase(ctx); err != nil {
		return err
	}

	environ.Notifiability = bbgo.Notifiability{
		SymbolChannelRouter:  bbgo.NewPatternChannelRouter(nil),
		SessionChannelRouter: bbgo.NewPatternChannelRouter(nil),
		ObjectChannelRouter:  bbgo.NewObjectChannelRouter(),
	}

	return nil
}

func BootstrapEnvironment(ctx context.Context, environ *bbgo.Environment, userConfig *bbgo.Config) error {
	if err := environ.ConfigureDatabase(ctx); err != nil {
		return err
	}

	if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
		return errors.Wrap(err, "exchange session configure error")
	}

	if userConfig.Persistence != nil {
		if err := environ.ConfigurePersistence(userConfig.Persistence); err != nil {
			return errors.Wrap(err, "persistence configure error")
		}
	}

	if err := environ.ConfigureNotificationSystem(userConfig); err != nil {
		return errors.Wrap(err, "notification configure error")
	}

	return nil
}

func runConfig(basectx context.Context, cmd *cobra.Command, userConfig *bbgo.Config) error {
	noSync, err := cmd.Flags().GetBool("no-sync")
	if err != nil {
		return err
	}

	enableWebServer, err := cmd.Flags().GetBool("enable-webserver")
	if err != nil {
		return err
	}

	webServerBind, err := cmd.Flags().GetString("webserver-bind")
	if err != nil {
		return err
	}

	enableWebServerLegacy, err := cmd.Flags().GetBool("enable-web-server")
	if err != nil {
		return err
	}
	if enableWebServerLegacy {
		log.Warn("command option --enable-web-server is renamed to --enable-webserver")
		enableWebServer = true
	}

	ctx, cancelTrading := context.WithCancel(basectx)
	defer cancelTrading()

	environ := bbgo.NewEnvironment()
	if err := BootstrapEnvironment(ctx, environ, userConfig); err != nil {
		return err
	}

	if err := environ.Init(ctx); err != nil {
		return err
	}

	if !noSync {
		if err := environ.Sync(ctx, userConfig); err != nil {
			return err
		}

		if userConfig.Sync != nil {
			environ.BindSync(userConfig.Sync)
		}
	}

	trader := bbgo.NewTrader(environ)
	if err := trader.Configure(userConfig); err != nil {
		return err
	}

	if err := trader.Run(ctx); err != nil {
		return err
	}

	if enableWebServer {
		go func() {
			s := &server.Server{
				Config:  userConfig,
				Environ: environ,
				Trader:  trader,
			}

			if err := s.Run(ctx, webServerBind); err != nil {
				log.WithError(err).Errorf("server error")
			}
		}()
	}

	cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
	cancelTrading()

	log.Infof("shutting down...")
	shutdownCtx, cancelShutdown := context.WithDeadline(ctx, time.Now().Add(30*time.Second))
	trader.Graceful.Shutdown(shutdownCtx)
	cancelShutdown()

	for _, session := range environ.Sessions() {
		if err := session.MarketDataStream.Close(); err != nil {
			log.WithError(err).Errorf("[%s] market data stream close error", session.Name)
		}
		if err := session.UserDataStream.Close(); err != nil {
			log.WithError(err).Errorf("[%s] user data stream close error", session.Name)
		}
	}

	return nil
}

func run(cmd *cobra.Command, args []string) error {
	setup, err := cmd.Flags().GetBool("setup")
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

	cpuProfile, err := cmd.Flags().GetString("cpu-profile")
	if err != nil {
		return err
	}

	if !setup {
		// if it's not setup, then the config file option is required.
		if len(configFile) == 0 {
			return errors.New("--config option is required")
		}

		if _, err := os.Stat(configFile); err != nil {
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
			return runSetup(ctx, userConfig, true)
		}

		// default setting is false, here load as true
		userConfig, err = bbgo.Load(configFile, true)
		if err != nil {
			return err
		}

		if cpuProfile != "" {
			f, err := os.Create(cpuProfile)
			if err != nil {
				log.Fatal("could not create CPU profile: ", err)
			}
			defer f.Close() // error handling omitted for example

			if err := pprof.StartCPUProfile(f); err != nil {
				log.Fatal("could not start CPU profile: ", err)
			}

			defer pprof.StopCPUProfile()
		}

		return runConfig(ctx, cmd, userConfig)
	}

	return runWrapperBinary(ctx, cmd, userConfig, args)
}

func runWrapperBinary(ctx context.Context, cmd *cobra.Command, userConfig *bbgo.Config, args []string) error {
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

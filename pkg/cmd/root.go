package cmd

import (
	"net/http"
	"os"
	"path"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/heroku/rollrus"
	"github.com/joho/godotenv"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/util"

	_ "time/tzdata"

	_ "github.com/go-sql-driver/mysql"
)

var cpuProfileFile *os.File

var userConfig *bbgo.Config

var RootCmd = &cobra.Command{
	Use:   "bbgo",
	Short: "bbgo is a crypto trading bot",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := cobraLoadDotenv(cmd, args); err != nil {
			return err
		}

		if viper.GetBool("debug") {
			log.Infof("debug mode is enabled")
			log.SetLevel(log.DebugLevel)
		}

		env := bbgo.GetCurrentEnv()

		logFormatter, err := cmd.Flags().GetString("log-formatter")
		if err != nil {
			return err
		}

		if len(logFormatter) == 0 {
			formatter := bbgo.NewLogFormatterWithEnv(env)
			log.SetFormatter(formatter)
		} else {
			formatter := bbgo.NewLogFormatter(bbgo.LogFormatterType(logFormatter))
			log.SetFormatter(formatter)
		}

		if token := viper.GetString("rollbar-token"); token != "" {
			log.Infof("found rollbar token %q, setting up rollbar hook...", util.MaskKey(token))

			log.AddHook(rollrus.NewHook(
				token,
				env,
			))
		}

		if viper.GetBool("metrics") {
			http.Handle("/metrics", promhttp.Handler())
			go func() {
				port := viper.GetString("metrics-port")
				log.Infof("starting metrics server at :%s", port)
				err := http.ListenAndServe(":"+port, nil)
				if err != nil {
					log.WithError(err).Errorf("metrics server error")
				}
			}()
		}

		cpuProfile, err := cmd.Flags().GetString("cpu-profile")
		if err != nil {
			return err
		}

		if cpuProfile != "" {
			log.Infof("starting cpu profiler, recording at %s", cpuProfile)

			cpuProfileFile, err = os.Create(cpuProfile)
			if err != nil {
				return errors.Wrap(err, "can not create file for CPU profile")
			}

			if err := pprof.StartCPUProfile(cpuProfileFile); err != nil {
				return errors.Wrap(err, "can not start CPU profile")
			}
		}

		return cobraLoadConfig(cmd, args)
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		pprof.StopCPUProfile()
		if cpuProfileFile != nil {
			return cpuProfileFile.Close() // error handling omitted for example
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func cobraLoadDotenv(cmd *cobra.Command, args []string) error {
	disableDotEnv, err := cmd.Flags().GetBool("no-dotenv")
	if err != nil {
		return err
	}

	if !disableDotEnv {
		dotenvFile, err := cmd.Flags().GetString("dotenv")
		if err != nil {
			return err
		}

		if _, err := os.Stat(dotenvFile); err == nil {
			if err := godotenv.Load(dotenvFile); err != nil {
				return errors.Wrap(err, "error loading dotenv file")
			}
		}
	}
	return nil
}

func cobraLoadConfig(cmd *cobra.Command, args []string) error {
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return errors.Wrapf(err, "failed to get the config flag")
	}

	// load config file nicely
	if len(configFile) > 0 {
		// if config file exists, use the config loaded from the config file.
		// otherwise, use an empty config object
		if _, err := os.Stat(configFile); err == nil {
			// load successfully
			userConfig, err = bbgo.Load(configFile, false)
			if err != nil {
				return errors.Wrapf(err, "can not load config file: %s", configFile)
			}

		} else if os.IsNotExist(err) {
			// config file doesn't exist, we should use the empty config
			userConfig = &bbgo.Config{}
		} else {
			// other error
			return errors.Wrapf(err, "config file load error: %s", configFile)
		}
	}

	return nil
}

func init() {
	RootCmd.PersistentFlags().Bool("debug", false, "debug mode")
	RootCmd.PersistentFlags().Bool("metrics", false, "enable prometheus metrics")
	RootCmd.PersistentFlags().String("metrics-port", "9090", "prometheus http server port")

	RootCmd.PersistentFlags().Bool("no-dotenv", false, "disable built-in dotenv")
	RootCmd.PersistentFlags().String("dotenv", ".env.local", "the dotenv file you want to load")

	RootCmd.PersistentFlags().String("config", "bbgo.yaml", "config file")

	RootCmd.PersistentFlags().String("log-formatter", "", "configure log formatter")

	RootCmd.PersistentFlags().String("rollbar-token", "", "rollbar token")

	// A flag can be 'persistent' meaning that this flag will be available to
	// the command it's assigned to as well as every command under that command.
	// For global flags, assign a flag as a persistent flag on the root.
	RootCmd.PersistentFlags().String("slack-token", "", "slack token")
	RootCmd.PersistentFlags().String("slack-channel", "dev-bbgo", "slack trading channel")
	RootCmd.PersistentFlags().String("slack-error-channel", "bbgo-error", "slack error channel")

	RootCmd.PersistentFlags().String("telegram-bot-token", "", "telegram bot token from bot father")
	RootCmd.PersistentFlags().String("telegram-bot-auth-token", "", "telegram auth token")

	RootCmd.PersistentFlags().String("binance-api-key", "", "binance api key")
	RootCmd.PersistentFlags().String("binance-api-secret", "", "binance api secret")

	RootCmd.PersistentFlags().String("max-api-key", "", "max api key")
	RootCmd.PersistentFlags().String("max-api-secret", "", "max api secret")

	RootCmd.PersistentFlags().String("cpu-profile", "", "cpu profile")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Enable environment variable binding, the env vars are not overloaded yet.
	viper.AutomaticEnv()

	// setup the config paths for looking up the config file
	/*
		viper.AddConfigPath("config")
		viper.AddConfigPath("$HOME/.bbgo")
		viper.AddConfigPath("/etc/bbgo")

		// set the config file name and format for loading the config file.
		viper.SetConfigName("bbgo")
		viper.SetConfigType("yaml")

		err := viper.ReadInConfig()
		if err != nil {
			log.WithError(err).Fatal("failed to load config file")
		}
	*/
	// Once the flags are defined, we can bind config keys with flags.
	if err := viper.BindPFlags(RootCmd.PersistentFlags()); err != nil {
		log.WithError(err).Errorf("failed to bind persistent flags. please check the flag settings.")
		return
	}

	environment := os.Getenv("BBGO_ENV")
	logDir := "log"
	switch environment {
	case "production", "prod":
		if err := os.MkdirAll(logDir, 0777); err != nil {
			log.Panic(err)
		}
		writer, err := rotatelogs.New(
			path.Join(logDir, "access_log.%Y%m%d"),
			rotatelogs.WithLinkName("access_log"),
			// rotatelogs.WithMaxAge(24 * time.Hour),
			rotatelogs.WithRotationTime(time.Duration(24)*time.Hour),
		)
		if err != nil {
			log.Panic(err)
		}

		log.AddHook(
			lfshook.NewHook(
				lfshook.WriterMap{
					log.DebugLevel: writer,
					log.InfoLevel:  writer,
					log.WarnLevel:  writer,
					log.ErrorLevel: writer,
					log.FatalLevel: writer,
				}, &log.JSONFormatter{}),
		)

	}
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.WithError(err).Fatalf("cannot execute command")
	}
}

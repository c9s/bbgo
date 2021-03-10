package cmd

import (
	"os"
	"path"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lestrrat-go/file-rotatelogs"
	"github.com/pkg/errors"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/x-cray/logrus-prefixed-formatter"

	_ "github.com/go-sql-driver/mysql"
)

var RootCmd = &cobra.Command{
	Use:   "bbgo",
	Short: "bbgo is a crypto trading bot",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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
	},

	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	RootCmd.PersistentFlags().Bool("debug", false, "debug flag")
	RootCmd.PersistentFlags().String("config", "bbgo.yaml", "config file")

	RootCmd.PersistentFlags().Bool("no-dotenv", false, "disable built-in dotenv")
	RootCmd.PersistentFlags().String("dotenv", ".env.local", "the dotenv file you want to load")

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

	RootCmd.PersistentFlags().String("ftx-api-key", "", "ftx api key")
	RootCmd.PersistentFlags().String("ftx-api-secret", "", "ftx api secret")
	RootCmd.PersistentFlags().String("ftx-subaccount-name", "", "subaccount name. Specify it if the credential is for subaccount.")
}

func Execute() {
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
	}

	if err := viper.BindPFlags(RootCmd.Flags()); err != nil {
		log.WithError(err).Errorf("failed to bind local flags. please check the flag settings.")
	}

	log.SetFormatter(&prefixed.TextFormatter{})

	logger := log.StandardLogger()
	if viper.GetBool("debug") {
		logger.SetLevel(log.DebugLevel)
	}

	environment := os.Getenv("BBGO_ENV")
	switch environment {
	case "production", "prod":

		writer, err := rotatelogs.New(
			path.Join("log", "access_log.%Y%m%d"),
			rotatelogs.WithLinkName("access_log"),
			// rotatelogs.WithMaxAge(24 * time.Hour),
			rotatelogs.WithRotationTime(time.Duration(24)*time.Hour),
		)
		if err != nil {
			log.Panic(err)
		}
		logger.AddHook(
			lfshook.NewHook(
				lfshook.WriterMap{
					log.DebugLevel: writer,
					log.InfoLevel:  writer,
					log.WarnLevel:  writer,
					log.ErrorLevel: writer,
					log.FatalLevel: writer,
				},
				&log.JSONFormatter{},
			),
		)
	}

	if err := RootCmd.Execute(); err != nil {
		log.WithError(err).Fatalf("cannot execute command")
	}
}

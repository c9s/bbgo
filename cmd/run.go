package cmd

import (
	"context"
	"syscall"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/config"
	"github.com/c9s/bbgo/pkg/slack/slacklog"
)

var errSlackTokenUndefined = errors.New("slack token is not defined.")

func init() {
	runCmd.Flags().String("config", "", "strategy config file")
	runCmd.Flags().String("since", "", "pnl since time")
	RootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "run strategies",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		if len(configFile) == 0 {
			return errors.New("--config option is not given")
		}

		userConfig, err := config.Load(configFile)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		slackToken := viper.GetString("slack-token")
		if len(slackToken) == 0 {
			return errSlackTokenUndefined
		}

		log.AddHook(slacklog.NewLogHook(slackToken, viper.GetString("slack-error-channel")))

		db, err := cmdutil.ConnectMySQL()
		if err != nil {
			return err
		}

		environ := bbgo.NewDefaultEnvironment(db)
		trader := bbgo.NewTrader(environ)

		for _, entry := range userConfig.ExchangeStrategies {
			for _, mount := range entry.Mounts {
				trader.AttachStrategyOn(mount, entry.Strategy)
			}
		}

		for _, strategy := range userConfig.CrossExchangeStrategies {
			trader.AttachCrossExchangeStrategy(strategy)
		}

		err = trader.Run(ctx)
		if err != nil {
			return err
		}

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return err
	},
}

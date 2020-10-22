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
	"github.com/c9s/bbgo/pkg/notifier/slacknotifier"
	"github.com/c9s/bbgo/pkg/slack/slacklog"

	_ "github.com/c9s/bbgo/pkg/strategy/buyandhold"
)

var errSlackTokenUndefined = errors.New("slack token is not defined.")

func init() {
	runCmd.Flags().String("config", "config/bbgo.yaml", "strategy config file")
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

		var notifier = slacknotifier.New(slackToken, viper.GetString("slack-channel"))

		db, err := cmdutil.ConnectMySQL()
		if err != nil {
			return err
		}

		environ := bbgo.NewDefaultEnvironment(db)
		environ.ReportTrade(notifier)

		trader := bbgo.NewTrader(environ)

		for _, entry := range userConfig.ExchangeStrategies {
			for _, mount := range entry.Mounts {
				log.Infof("attaching strategy %T on %s...", entry.Strategy, mount)
				trader.AttachStrategyOn(mount, entry.Strategy)
			}
		}

		for _, strategy := range userConfig.CrossExchangeStrategies {
			log.Infof("attaching strategy %T", strategy)
			trader.AttachCrossExchangeStrategy(strategy)
		}

		// TODO: load these from config file
		trader.ReportPnL(notifier).
			AverageCostBySymbols("BTCUSDT", "BNBUSDT").
			Of("binance").When("@daily", "@hourly")

		trader.ReportPnL(notifier).
			AverageCostBySymbols("MAXUSDT").
			Of("max").When("@daily", "@hourly")

		err = trader.Run(ctx)
		if err != nil {
			return err
		}

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return err
	},
}

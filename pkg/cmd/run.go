package cmd

import (
	"context"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/config"
	"github.com/c9s/bbgo/pkg/notifier/slacknotifier"
	"github.com/c9s/bbgo/pkg/slack/slacklog"
)

var errSlackTokenUndefined = errors.New("slack token is not defined.")

func init() {
	RunCmd.Flags().String("config", "config/bbgo.yaml", "strategy config file")
	RunCmd.Flags().String("since", "", "pnl since time")
	RootCmd.AddCommand(RunCmd)
}

var RunCmd = &cobra.Command{
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

		logrus.AddHook(slacklog.NewLogHook(slackToken, viper.GetString("slack-error-channel")))

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
				logrus.Infof("attaching strategy %T on %s...", entry.Strategy, mount)
				trader.AttachStrategyOn(mount, entry.Strategy)
			}
		}

		for _, strategy := range userConfig.CrossExchangeStrategies {
			logrus.Infof("attaching strategy %T", strategy)
			trader.AttachCrossExchangeStrategy(strategy)
		}

		for _, report := range userConfig.PnLReporters {
			if len(report.AverageCostBySymbols) > 0 {
				trader.ReportPnL(notifier).
					AverageCostBySymbols(report.AverageCostBySymbols...).
					Of(report.Of...).
					When(report.When...)
			} else {
				return errors.Errorf("unsupported PnL reporter: %+v", report)
			}
		}

		err = trader.Run(ctx)
		if err != nil {
			return err
		}

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return err
	},
}

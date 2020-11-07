package cmd

import (
	"context"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/backtest"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	BacktestCmd.Flags().String("exchange", "", "target exchange")
	BacktestCmd.Flags().Bool("sync", false, "sync backtest data")
	BacktestCmd.Flags().String("config", "config/bbgo.yaml", "strategy config file")
	RootCmd.AddCommand(BacktestCmd)
}

var BacktestCmd = &cobra.Command{
	Use:          "backtest",
	Short:        "backtest your strategies",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		if len(configFile) == 0 {
			return errors.New("--config option is required")
		}

		wantSync, err := cmd.Flags().GetBool("sync")
		if err != nil {
			return err
		}

		exchangeNameStr, err := cmd.Flags().GetString("exchange")
		if err != nil {
			return err
		}

		exchangeName, err := types.ValidExchangeName(exchangeNameStr)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		userConfig, err := bbgo.Load(configFile)
		if err != nil {
			return err
		}

		db, err := cmdutil.ConnectMySQL()
		if err != nil {
			return err
		}

		if userConfig.Backtest == nil {
			return errors.New("backtest config is not defined")
		}

		// set default start time to the past 6 months
		startTime := time.Now().AddDate(0, -6, 0)
		if len(userConfig.Backtest.StartTime) == 0 {
			userConfig.Backtest.StartTime = startTime.Format("2006-01-02")
		}

		backtestService := &service.BacktestService{DB: db}

		exchange := backtest.NewExchange(exchangeName, backtestService, userConfig.Backtest)

		if wantSync {
			for _, symbol := range userConfig.Backtest.Symbols {
				if err := backtestService.Sync(ctx, exchange, symbol, startTime); err != nil {
					return err
				}
			}
		}

		environ := bbgo.NewEnvironment()
		environ.AddExchange(exchangeName.String(), exchange)

		environ.Notifiability = bbgo.Notifiability{
			SymbolChannelRouter:  bbgo.NewPatternChannelRouter(nil),
			SessionChannelRouter: bbgo.NewPatternChannelRouter(nil),
			ObjectChannelRouter:  bbgo.NewObjectChannelRouter(),
		}

		trader := bbgo.NewTrader(environ)
		if userConfig.RiskControls != nil {
			trader.SetRiskControls(userConfig.RiskControls)
		}

		for _, entry := range userConfig.ExchangeStrategies {
			log.Infof("attaching strategy %T on %s instead of %v", entry.Strategy, exchangeName.String(), entry.Mounts)
			trader.AttachStrategyOn(exchangeName.String(), entry.Strategy)
		}

		if len(userConfig.CrossExchangeStrategies) > 0 {
			log.Warnf("backtest does not support CrossExchangeStrategy, strategies won't be added.")
		}

		if err := trader.Run(ctx); err != nil {
			return err
		}

		<-exchange.Done()

		for _, session := range environ.Sessions() {
			calculator := &pnl.AverageCostCalculator{
				TradingFeeCurrency: exchange.PlatformFeeCurrency(),
			}
			for symbol, trades := range session.Trades {
				lastPrice, ok := session.LastPrice(symbol)
				if !ok {
					return errors.Errorf("last price not found: %s", symbol)
				}

				report := calculator.Calculate(symbol, trades, lastPrice)
				report.Print()
			}
		}

		return nil
	},
}

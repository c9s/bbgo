package cmd

import (
	"context"
	"fmt"
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
	BacktestCmd.Flags().Bool("base-asset-baseline", false, "use base asset performance as the competitive baseline performance")
	BacktestCmd.Flags().CountP("verbose", "v", "verbose level")
	BacktestCmd.Flags().String("config", "config/bbgo.yaml", "strategy config file")
	RootCmd.AddCommand(BacktestCmd)
}

var BacktestCmd = &cobra.Command{
	Use:          "backtest",
	Short:        "backtest your strategies",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		verboseCnt, err := cmd.Flags().GetCount("verbose")
		if err != nil {
			return err
		}

		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		if len(configFile) == 0 {
			return errors.New("--config option is required")
		}

		wantBaseAssetBaseline, err := cmd.Flags().GetBool("base-asset-baseline")
		if err != nil {
			return err
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

		exchange, err := cmdutil.NewExchange(exchangeName)
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
		if len(userConfig.Backtest.StartTime) == 0 {
			userConfig.Backtest.StartTime = time.Now().AddDate(0, -6, 0).Format("2006-01-02")
		}

		startTime, err := userConfig.Backtest.ParseStartTime()
		if err != nil {
			return err
		}

		backtestService := &service.BacktestService{DB: db}

		if wantSync {
			for _, symbol := range userConfig.Backtest.Symbols {
				if err := backtestService.Sync(ctx, exchange, symbol, startTime); err != nil {
					return err
				}
			}
		}

		backtestExchange := backtest.NewExchange(exchangeName, backtestService, userConfig.Backtest)

		environ := bbgo.NewEnvironment()
		environ.SetStartTime(startTime)
		environ.AddExchange(exchangeName.String(), backtestExchange)

		environ.Notifiability = bbgo.Notifiability{
			SymbolChannelRouter:  bbgo.NewPatternChannelRouter(nil),
			SessionChannelRouter: bbgo.NewPatternChannelRouter(nil),
			ObjectChannelRouter:  bbgo.NewObjectChannelRouter(),
		}

		trader := bbgo.NewTrader(environ)

		if verboseCnt == 2 {
			log.SetLevel(log.DebugLevel)
		} else if verboseCnt > 0 {
			log.SetLevel(log.InfoLevel)
		} else {
			// default mode, disable strategy logging and order executor logging
			log.SetLevel(log.ErrorLevel)
			trader.DisableLogging()
		}

		if userConfig.RiskControls != nil {
			log.Infof("setting risk controls: %+v", userConfig.RiskControls)
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

		<-backtestExchange.Done()

		// put the logger back to print the pnl
		log.SetLevel(log.InfoLevel)
		for _, session := range environ.Sessions() {

			calculator := &pnl.AverageCostCalculator{
				TradingFeeCurrency: backtestExchange.PlatformFeeCurrency(),
			}
			for symbol, trades := range session.Trades {
				market, ok := session.Market(symbol)
				if !ok {
					return fmt.Errorf("market not found: %s", symbol)
				}

				startPrice, ok := session.StartPrice(symbol)
				if !ok {
					return fmt.Errorf("start price not found: %s", symbol)
				}

				log.Infof("%s PROFIT AND LOSS REPORT", symbol)
				log.Infof("===============================================")

				lastPrice, ok := session.LastPrice(symbol)
				if !ok {
					return fmt.Errorf("last price not found: %s", symbol)
				}

				report := calculator.Calculate(symbol, trades, lastPrice)
				report.Print()

				initBalances := userConfig.Backtest.Account.Balances.BalanceMap()
				finalBalances := session.Account.Balances()

				log.Infof("INITIAL BALANCES:")
				initBalances.Print()

				log.Infof("FINAL BALANCES:")
				finalBalances.Print()

				if wantBaseAssetBaseline {
					initBaseAsset := InBaseAsset(initBalances, market, startPrice)
					finalBaseAsset := InBaseAsset(finalBalances, market, lastPrice)
					log.Infof("INITIAL ASSET ~= %s %s (1 %s = %f)", market.FormatQuantity(initBaseAsset), market.BaseCurrency, market.BaseCurrency, startPrice)
					log.Infof("FINAL ASSET ~= %s %s (1 %s = %f)", market.FormatQuantity(finalBaseAsset), market.BaseCurrency, market.BaseCurrency, lastPrice)

					log.Infof("%s BASE ASSET PERFORMANCE: %.2f%% (= (%.2f - %.2f) / %.2f)", symbol, (finalBaseAsset-initBaseAsset)/initBaseAsset*100.0, finalBaseAsset, initBaseAsset, initBaseAsset)
					log.Infof("%s PERFORMANCE: %.2f%% (= (%.2f - %.2f) / %.2f)", symbol, (lastPrice-startPrice)/startPrice*100.0, lastPrice, startPrice, startPrice)
				}
			}
		}

		return nil
	},
}

func InBaseAsset(balances types.BalanceMap, market types.Market, price float64) float64 {
	quote := balances[market.QuoteCurrency]
	base := balances[market.BaseCurrency]
	return (base.Locked.Float64() + base.Available.Float64()) + ((quote.Locked.Float64() + quote.Available.Float64()) / price)
}

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/backtest"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	BacktestCmd.Flags().String("exchange", "", "target exchange")
	BacktestCmd.Flags().Bool("sync", false, "sync backtest data")
	BacktestCmd.Flags().Bool("sync-only", false, "sync backtest data only, do not run backtest")
	BacktestCmd.Flags().String("sync-from", "", "sync backtest data from the given time, which will override the time range in the backtest config")
	BacktestCmd.Flags().Bool("base-asset-baseline", false, "use base asset performance as the competitive baseline performance")
	BacktestCmd.Flags().CountP("verbose", "v", "verbose level")
	BacktestCmd.Flags().String("config", "config/bbgo.yaml", "strategy config file")
	BacktestCmd.Flags().Bool("force", false, "force execution without confirm")
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

		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return err
		}

		syncOnly, err := cmd.Flags().GetBool("sync-only")
		if err != nil {
			return err
		}

		syncFromDateStr, err := cmd.Flags().GetString("sync-from")
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

		sourceExchange, err := cmdutil.NewExchange(exchangeName)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		userConfig, err := bbgo.Load(configFile, true)
		if err != nil {
			return err
		}

		if userConfig.Backtest == nil {
			return errors.New("backtest config is not defined")
		}

		now := time.Now()
		// set default start time to the past 6 months
		if len(userConfig.Backtest.StartTime) == 0 {
			userConfig.Backtest.StartTime = now.AddDate(0, -6, 0).Format("2006-01-02")
		}
		if len(userConfig.Backtest.EndTime) == 0 {
			userConfig.Backtest.EndTime = now.Format("2006-01-02")
		}

		if len(userConfig.CrossExchangeStrategies) > 0 {
			log.Warnf("backtest does not support CrossExchangeStrategy, strategies won't be added.")
		}

		startTime, err := userConfig.Backtest.ParseStartTime()
		if err != nil {
			return err
		}

		log.Infof("starting backtest with startTime %s", startTime.Format(time.ANSIC))

		environ := bbgo.NewEnvironment()
		if err := BootstrapBacktestEnvironment(ctx, environ, userConfig); err != nil {
			return err
		}

		if environ.DatabaseService == nil {
			return errors.New("database service is not enabled, please check your environment variables DB_DRIVER and DB_DSN")
		}

		backtestService := &service.BacktestService{DB: environ.DatabaseService.DB}
		environ.BacktestService = backtestService

		if wantSync {
			var syncFromTime = startTime

			// override the sync from time if the option is given
			if len(syncFromDateStr) > 0 {
				syncFromTime, err = time.Parse(types.DateFormat, syncFromDateStr)
				if err != nil {
					return err
				}

				if syncFromTime.After(startTime) {
					return fmt.Errorf("sync-from time %s can not be latter than the backtest start time %s", syncFromTime, startTime)
				}
			} else {
				// we need at least 1 month backward data for EMA and last prices
				syncFromTime = syncFromTime.AddDate(0, -1, 0)
				log.Infof("adjusted sync start time to %s for backward market data", syncFromTime)
			}

			log.Info("starting synchronization...")
			for _, symbol := range userConfig.Backtest.Symbols {
				if err := backtestService.Sync(ctx, sourceExchange, symbol, syncFromTime); err != nil {
					return err
				}
			}
			log.Info("synchronization done")

			var corruptCnt = 0
			for _, symbol := range userConfig.Backtest.Symbols {
				log.Infof("verifying backtesting data...")

				for interval := range types.SupportedIntervals {
					log.Infof("verifying %s %s kline data...", symbol, interval)

					klineC, errC := backtestService.QueryKLinesCh(startTime, time.Now(), sourceExchange, []string{symbol}, []types.Interval{interval})
					var emptyKLine types.KLine
					var prevKLine types.KLine
					for k := range klineC {
						if verboseCnt > 1 {
							fmt.Print(".")
						}

						if prevKLine != emptyKLine {
							if prevKLine.StartTime.Add(interval.Duration()) != k.StartTime {
								corruptCnt++
								log.Errorf("found kline data corrupted at time: %s kline: %+v", k.StartTime, k)
								log.Errorf("between %d and %d",
									prevKLine.StartTime.Unix(),
									k.StartTime.Unix())
							}
						}

						prevKLine = k
					}

					if verboseCnt > 1 {
						fmt.Println()
					}

					if err := <-errC; err != nil {
						return err
					}
				}
			}

			log.Infof("backtest verification completed")
			if corruptCnt > 0 {
				log.Errorf("found %d corruptions", corruptCnt)
			} else {
				log.Infof("found %d corruptions", corruptCnt)
			}

			if syncOnly {
				return nil
			}
		}

		log.Warn("!!! To run backtest, you should use an isolated database for storing backtest trades !!!")
		log.Warn("!!! The trade record in the current database WILL ALL BE DELETE !!!")

		if !force {
			if !confirmation("Are you sure to continue?") {
				return nil
			}
		}

		if err := environ.TradeService.DeleteAll(); err != nil {
			return err
		}

		backtestExchange := backtest.NewExchange(exchangeName, backtestService, userConfig.Backtest)
		environ.SetStartTime(startTime)
		environ.AddExchange(exchangeName.String(), backtestExchange)

		if err := environ.Init(ctx); err != nil {
			return err
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

		if err := trader.Configure(userConfig); err != nil {
			return err
		}

		if err := trader.Run(ctx); err != nil {
			return err
		}

		<-backtestExchange.Done()

		log.Infof("shutting down trader...")
		shutdownCtx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
		trader.Graceful.Shutdown(shutdownCtx)
		cancel()

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

				report := calculator.Calculate(symbol, trades.Trades, lastPrice)
				report.Print()

				initBalances := userConfig.Backtest.Account.Balances.BalanceMap()
				finalBalances := session.Account.Balances()

				log.Infof("INITIAL BALANCES:")
				initBalances.Print()

				log.Infof("FINAL BALANCES:")
				finalBalances.Print()

				if wantBaseAssetBaseline {
					initBaseAsset := inBaseAsset(initBalances, market, startPrice)
					finalBaseAsset := inBaseAsset(finalBalances, market, lastPrice)
					log.Infof("INITIAL ASSET ~= %s %s (1 %s = %f)", market.FormatQuantity(initBaseAsset), market.BaseCurrency, market.BaseCurrency, startPrice)
					log.Infof("FINAL ASSET ~= %s %s (1 %s = %f)", market.FormatQuantity(finalBaseAsset), market.BaseCurrency, market.BaseCurrency, lastPrice)

					log.Infof("%s BASE ASSET PERFORMANCE: %.2f%% (= (%.2f - %.2f) / %.2f)", market.BaseCurrency, (finalBaseAsset-initBaseAsset)/initBaseAsset*100.0, finalBaseAsset, initBaseAsset, initBaseAsset)
					log.Infof("%s PERFORMANCE: %.2f%% (= (%.2f - %.2f) / %.2f)", market.BaseCurrency, (lastPrice-startPrice)/startPrice*100.0, lastPrice, startPrice, startPrice)
				}
			}
		}

		return nil
	},
}

func confirmation(s string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/N]: ", s)

		response, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		} else {
			return false
		}
	}
}

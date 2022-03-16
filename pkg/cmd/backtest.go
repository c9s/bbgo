package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/backtest"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

type BackTestReport struct {
	Symbol          string                    `json:"symbol,omitempty"`
	LastPrice       fixedpoint.Value          `json:"lastPrice,omitempty"`
	StartPrice      fixedpoint.Value          `json:"startPrice,omitempty"`
	PnLReport       *pnl.AverageCostPnlReport `json:"pnlReport,omitempty"`
	InitialBalances types.BalanceMap          `json:"initialBalances,omitempty"`
	FinalBalances   types.BalanceMap          `json:"finalBalances,omitempty"`
}

func init() {
	BacktestCmd.Flags().Bool("sync", false, "sync backtest data")
	BacktestCmd.Flags().Bool("sync-only", false, "sync backtest data only, do not run backtest")
	BacktestCmd.Flags().String("sync-from", "", "sync backtest data from the given time, which will override the time range in the backtest config")
	BacktestCmd.Flags().Bool("verify", false, "verify the kline back-test data")

	BacktestCmd.Flags().Bool("base-asset-baseline", false, "use base asset performance as the competitive baseline performance")
	BacktestCmd.Flags().CountP("verbose", "v", "verbose level")
	BacktestCmd.Flags().String("config", "config/bbgo.yaml", "strategy config file")
	BacktestCmd.Flags().Bool("force", false, "force execution without confirm")
	BacktestCmd.Flags().String("output", "", "the report output directory")
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

		if viper.GetBool("debug") {
			verboseCnt = 2
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

		outputDirectory, err := cmd.Flags().GetString("output")
		if err != nil {
			return err
		}

		jsonOutputEnabled := len(outputDirectory) > 0

		syncOnly, err := cmd.Flags().GetBool("sync-only")
		if err != nil {
			return err
		}

		syncFromDateStr, err := cmd.Flags().GetString("sync-from")
		if err != nil {
			return err
		}

		shouldVerify, err := cmd.Flags().GetBool("verify")
		if err != nil {
			return err
		}

		userConfig, err := bbgo.Load(configFile, true)
		if err != nil {
			return err
		}

		if verboseCnt == 2 {
			log.SetLevel(log.DebugLevel)
		} else if verboseCnt > 0 {
			log.SetLevel(log.InfoLevel)
		} else {
			// default mode, disable strategy logging and order executor logging
			log.SetLevel(log.ErrorLevel)
		}

		if userConfig.Backtest == nil {
			return errors.New("backtest config is not defined")
		}

		acceptAllSessions := false
		var whitelistedSessions map[string]struct{}
		if len(userConfig.Backtest.Sessions) == 0 {
			acceptAllSessions = true
		} else {
			for _, name := range userConfig.Backtest.Sessions {
				_, err := types.ValidExchangeName(name)
				if err != nil {
					return err
				}
				whitelistedSessions[name] = struct{}{}
			}
		}

		sourceExchanges := make(map[string]types.Exchange)

		for key, session := range userConfig.Sessions {
			ok := acceptAllSessions
			if !ok {
				_, ok = whitelistedSessions[key]
			}
			if ok {
				publicExchange, err := cmdutil.NewExchangePublic(session.ExchangeName)
				if err != nil {
					return err
				}

				sourceExchanges[key] = publicExchange
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var now = time.Now()
		var startTime, endTime time.Time

		startTime = userConfig.Backtest.StartTime.Time()

		// set default start time to the past 6 months
		// userConfig.Backtest.StartTime = now.AddDate(0, -6, 0).Format("2006-01-02")

		if userConfig.Backtest.EndTime != nil {
			endTime = userConfig.Backtest.EndTime.Time()
		} else {
			endTime = now
		}
		_ = endTime

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
			var syncFromTime time.Time

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
				syncFromTime = startTime.AddDate(0, -1, 0)
				log.Infof("adjusted sync start time %s to %s for backward market data", startTime, syncFromTime)
			}

			log.Info("starting synchronization...")
			for _, symbol := range userConfig.Backtest.Symbols {

				for _, sourceExchange := range sourceExchanges {
					exCustom, ok := sourceExchange.(types.CustomIntervalProvider)

					var supportIntervals map[types.Interval]int
					if ok {
						supportIntervals = exCustom.SupportedInterval()
					} else {
						supportIntervals = types.SupportedIntervals
					}

					for interval := range supportIntervals {
						// if err := s.SyncKLineByInterval(ctx, exchange, symbol, interval, startTime, endTime); err != nil {
						//	return err
						// }
						firstKLine, err := backtestService.QueryFirstKLine(sourceExchange.Name(), symbol, interval)
						if err != nil {
							return errors.Wrapf(err, "failed to query backtest kline")
						}

						// if we don't have klines before the start time endpoint, the back-test will fail.
						// because the last price will be missing.
						if firstKLine != nil {
							if err := backtestService.SyncExist(ctx, sourceExchange, symbol, syncFromTime, time.Now(), interval); err != nil {
								return err
							}
						} else {
							if err := backtestService.Sync(ctx, sourceExchange, symbol, syncFromTime, time.Now(), interval); err != nil {
								return err
							}
						}
					}
				}
			}
			log.Info("synchronization done")

			for _, sourceExchange := range sourceExchanges {
				if shouldVerify {
					err2, done := backtestService.Verify(userConfig.Backtest.Symbols, startTime, time.Now(), sourceExchange, verboseCnt)
					if done {
						return err2
					}
				}
			}

			if syncOnly {
				return nil
			}
		}

		if userConfig.Backtest.RecordTrades {
			log.Warn("!!! Trade recording is enabled for back-testing !!!")
			log.Warn("!!! To run back-testing, you should use an isolated database for storing back-testing trades !!!")
			log.Warn("!!! The trade record in the current database WILL ALL BE DELETED BEFORE THIS BACK-TESTING !!!")
			if !force {
				if !confirmation("Are you sure to continue?") {
					return nil
				}
			}

			if err := environ.TradeService.DeleteAll(); err != nil {
				return err
			}
		}

		environ.SetStartTime(startTime)

		// exchangeNameStr is the session name.
		for name, sourceExchange := range sourceExchanges {

			backtestExchange, err := backtest.NewExchange(sourceExchange.Name(), sourceExchange, backtestService, userConfig.Backtest)
			if err != nil {
				return errors.Wrap(err, "failed to create backtest exchange")
			}
			environ.AddExchange(name, backtestExchange)
		}

		if err := environ.Init(ctx); err != nil {
			return err
		}

		trader := bbgo.NewTrader(environ)
		if verboseCnt == 0 {
			trader.DisableLogging()
		}

		if err := trader.Configure(userConfig); err != nil {
			return err
		}

		if err := trader.Run(ctx); err != nil {
			return err
		}

		type KChanEx struct {
			KChan    chan types.KLine
			Exchange *backtest.Exchange
		}
		for _, session := range environ.Sessions() {
			backtestExchange := session.Exchange.(*backtest.Exchange)
			backtestExchange.InitMarketData()
		}

		var klineChans []KChanEx
		for _, session := range environ.Sessions() {
			exchange := session.Exchange.(*backtest.Exchange)
			c, err := exchange.GetMarketData()
			if err != nil {
				return err
			}
			klineChans = append(klineChans, KChanEx{KChan: c, Exchange: exchange})
		}

		runCtx, cancelRun := context.WithCancel(ctx)
		go func() {
			defer cancelRun()
			for {
				count := len(klineChans)
				for _, kchanex := range klineChans {
					kLine, more := <-kchanex.KChan
					if more {
						kchanex.Exchange.ConsumeKLine(kLine)
					} else {
						if err := kchanex.Exchange.CloseMarketData(); err != nil {
							log.Errorf("%v", err)
							return
						}
						count--
					}
				}
				if count == 0 {
					break
				}
			}
		}()

		cmdutil.WaitForSignal(runCtx, syscall.SIGINT, syscall.SIGTERM)

		log.Infof("shutting down trader...")
		shutdownCtx, cancelShutdown := context.WithDeadline(runCtx, time.Now().Add(10*time.Second))
		trader.Graceful.Shutdown(shutdownCtx)
		cancelShutdown()

		// put the logger back to print the pnl
		log.SetLevel(log.InfoLevel)
		for _, session := range environ.Sessions() {
			backtestExchange := session.Exchange.(*backtest.Exchange)
			exchangeName := session.Exchange.Name().String()
			for symbol, trades := range session.Trades {
				market, ok := session.Market(symbol)
				if !ok {
					return fmt.Errorf("market not found: %s, %s", symbol, exchangeName)
				}

				calculator := &pnl.AverageCostCalculator{
					TradingFeeCurrency: backtestExchange.PlatformFeeCurrency(),
					Market:             market,
				}

				startPrice, ok := session.StartPrice(symbol)
				if !ok {
					return fmt.Errorf("start price not found: %s, %s", symbol, exchangeName)
				}

				lastPrice, ok := session.LastPrice(symbol)
				if !ok {
					return fmt.Errorf("last price not found: %s, %s", symbol, exchangeName)
				}

				color.Green("%s %s PROFIT AND LOSS REPORT", strings.ToUpper(exchangeName), symbol)
				color.Green("===============================================")

				report := calculator.Calculate(symbol, trades.Trades, lastPrice)
				report.Print()

				initBalances := userConfig.Backtest.Account[exchangeName].Balances.BalanceMap()
				finalBalances := session.Account.Balances()

				log.Infof("INITIAL BALANCES:")
				initBalances.Print()

				log.Infof("FINAL BALANCES:")
				finalBalances.Print()

				if jsonOutputEnabled {
					result := BackTestReport{
						Symbol:          symbol,
						LastPrice:       lastPrice,
						StartPrice:      startPrice,
						PnLReport:       report,
						InitialBalances: initBalances,
						FinalBalances:   finalBalances,
					}

					jsonOutput, err := json.MarshalIndent(&result, "", "  ")
					if err != nil {
						return err
					}

					if err := ioutil.WriteFile(filepath.Join(outputDirectory, symbol+".json"), jsonOutput, 0644); err != nil {
						return err
					}
				}

				initQuoteAsset := inQuoteAsset(initBalances, market, startPrice)
				finalQuoteAsset := inQuoteAsset(finalBalances, market, lastPrice)
				log.Infof("INITIAL ASSET IN %s ~= %s %s (1 %s = %v)", market.QuoteCurrency, market.FormatQuantity(initQuoteAsset), market.QuoteCurrency, market.BaseCurrency, startPrice)
				log.Infof("FINAL ASSET IN %s ~= %s %s (1 %s = %v)", market.QuoteCurrency, market.FormatQuantity(finalQuoteAsset), market.QuoteCurrency, market.BaseCurrency, lastPrice)

				if report.Profit.Sign() > 0 {
					color.Green("REALIZED PROFIT: +%v %s", report.Profit, market.QuoteCurrency)
				} else {
					color.Red("REALIZED PROFIT: %v %s", report.Profit, market.QuoteCurrency)
				}

				if report.UnrealizedProfit.Sign() > 0 {
					color.Green("UNREALIZED PROFIT: +%v %s", report.UnrealizedProfit, market.QuoteCurrency)
				} else {
					color.Red("UNREALIZED PROFIT: %v %s", report.UnrealizedProfit, market.QuoteCurrency)
				}

				if finalQuoteAsset.Compare(initQuoteAsset) > 0 {
					color.Green("ASSET INCREASED: +%v %s (+%s)", finalQuoteAsset.Sub(initQuoteAsset), market.QuoteCurrency, finalQuoteAsset.Sub(initQuoteAsset).Div(initQuoteAsset).FormatPercentage(2))
				} else {
					color.Red("ASSET DECREASED: %v %s (%s)", finalQuoteAsset.Sub(initQuoteAsset), market.QuoteCurrency, finalQuoteAsset.Sub(initQuoteAsset).Div(initQuoteAsset).FormatPercentage(2))
				}

				if wantBaseAssetBaseline {
					// initBaseAsset := inBaseAsset(initBalances, market, startPrice)
					// finalBaseAsset := inBaseAsset(finalBalances, market, lastPrice)
					// log.Infof("INITIAL ASSET IN %s ~= %s %s (1 %s = %f)", market.BaseCurrency, market.FormatQuantity(initBaseAsset), market.BaseCurrency, market.BaseCurrency, startPrice)
					// log.Infof("FINAL ASSET IN %s ~= %s %s (1 %s = %f)", market.BaseCurrency, market.FormatQuantity(finalBaseAsset), market.BaseCurrency, market.BaseCurrency, lastPrice)

					if lastPrice.Compare(startPrice) > 0 {
						color.Green("%s BASE ASSET PERFORMANCE: +%s (= (%s - %s) / %s)",
							market.BaseCurrency,
							lastPrice.Sub(startPrice).Div(startPrice).FormatPercentage(2),
							lastPrice.FormatString(2),
							startPrice.FormatString(2),
							startPrice.FormatString(2))
					} else {
						color.Red("%s BASE ASSET PERFORMANCE: %s (= (%s - %s) / %s)",
							market.BaseCurrency,
							lastPrice.Sub(startPrice).Div(startPrice).FormatPercentage(2),
							lastPrice.FormatString(2),
							startPrice.FormatString(2),
							startPrice.FormatString(2))
					}
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

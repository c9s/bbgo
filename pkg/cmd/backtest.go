package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/backtest"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/data/tsv"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func init() {
	BacktestCmd.Flags().Bool("sync", false, "sync backtest data")
	BacktestCmd.Flags().Bool("sync-only", false, "sync backtest data only, do not run backtest")
	BacktestCmd.Flags().String("sync-from", "", "sync backtest data from the given time, which will override the time range in the backtest config")
	BacktestCmd.Flags().String("sync-exchange", "", "specify only one exchange to sync backtest data")
	BacktestCmd.Flags().String("session", "", "specify only one exchange session to run backtest")

	BacktestCmd.Flags().Bool("verify", false, "verify the kline back-test data")

	BacktestCmd.Flags().Bool("base-asset-baseline", false, "use base asset performance as the competitive baseline performance")
	BacktestCmd.Flags().CountP("verbose", "v", "verbose level")
	BacktestCmd.Flags().String("config", "config/bbgo.yaml", "strategy config file")
	BacktestCmd.Flags().Bool("force", false, "force execution without confirm")
	BacktestCmd.Flags().String("output", "", "the report output directory")
	BacktestCmd.Flags().Bool("subdir", false, "generate report in the sub-directory of the output directory")
	RootCmd.AddCommand(BacktestCmd)
}

var BacktestCmd = &cobra.Command{
	Use:          "backtest",
	Short:        "run backtest with strategies",
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

		syncExchangeName, err := cmd.Flags().GetString("sync-exchange")
		if err != nil {
			return err
		}

		sessionName, err := cmd.Flags().GetString("session")
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

		generatingReport := len(outputDirectory) > 0

		reportFileInSubDir, err := cmd.Flags().GetBool("subdir")
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

		if len(sessionName) > 0 {
			userConfig.Backtest.Sessions = []string{sessionName}
		} else if len(syncExchangeName) > 0 {
			userConfig.Backtest.Sessions = []string{syncExchangeName}
		} else if len(userConfig.Backtest.Sessions) == 0 {
			log.Infof("backtest.sessions is not defined, using all supported exchanges: %v", types.SupportedExchanges)
			for _, exName := range types.SupportedExchanges {
				userConfig.Backtest.Sessions = append(userConfig.Backtest.Sessions, exName.String())
			}
		}

		var sourceExchanges = make(map[types.ExchangeName]types.Exchange)
		for _, name := range userConfig.Backtest.Sessions {
			exName, err := types.ValidExchangeName(name)
			if err != nil {
				return err
			}

			publicExchange, err := cmdutil.NewExchangePublic(exName)
			if err != nil {
				return err
			}
			sourceExchanges[exName] = publicExchange
		}

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

			log.Infof("starting synchronization: %v", userConfig.Backtest.Symbols)
			if err := sync(ctx, userConfig, backtestService, sourceExchanges, syncFromTime); err != nil {
				return err
			}
			log.Info("synchronization done")

			if shouldVerify {
				err := verify(userConfig, backtestService, sourceExchanges, startTime, verboseCnt)
				if err != nil {
					return err
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
			environ.AddExchange(name.String(), backtestExchange)
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

		exchangeSources, err := toExchangeSources(environ.Sessions())
		if err != nil {
			return err
		}

		// back-test session report name
		/*
			var backtestSessionName = backtest.FormatSessionName(
				userConfig.Backtest.Sessions,
				userConfig.Backtest.Symbols,
				userConfig.Backtest.StartTime.Time(),
				userConfig.Backtest.EndTime.Time(),
			)
		*/

		var kLineHandlers []func(k types.KLine, exSource *backtest.ExchangeDataSource)
		var manifests backtest.Manifests
		var runID = userConfig.GetSignature() + "_" + uuid.NewString()
		var reportDir = outputDirectory

		if generatingReport {
			if reportFileInSubDir {
				// reportDir = filepath.Join(reportDir, backtestSessionName)
				reportDir = filepath.Join(reportDir, runID)
			}

			kLineDataDir := filepath.Join(reportDir, "klines")
			if err := util.SafeMkdirAll(kLineDataDir); err != nil {
				return err
			}

			stateRecorder := backtest.NewStateRecorder(reportDir)
			err = trader.IterateStrategies(func(st bbgo.StrategyID) error {
				return stateRecorder.Scan(st.(backtest.Instance))
			})
			manifests = stateRecorder.Manifests()

			if err != nil {
				return err
			}

			// state snapshot
			kLineHandlers = append(kLineHandlers, func(k types.KLine, _ *backtest.ExchangeDataSource) {
				// snapshot per 1m
				if k.Interval == types.Interval1m && k.Closed {
					if _, err := stateRecorder.Snapshot(); err != nil {
						log.WithError(err).Errorf("state record failed to snapshot the strategy state")
					}
				}
			})

			dumper := backtest.NewKLineDumper(kLineDataDir)
			defer func() {
				_ = dumper.Close()
			}()
			defer func() {
				if err := dumper.Close(); err != nil {
					log.WithError(err).Errorf("kline dumper can not close files")
				}
			}()

			kLineHandlers = append(kLineHandlers, func(k types.KLine, _ *backtest.ExchangeDataSource) {
				if err := dumper.Record(k); err != nil {
					log.WithError(err).Errorf("can not write kline to file")
				}
			})

			// equity curve recording -- record per 1h kline
			equityCurveTsv, err := tsv.NewWriterFile(filepath.Join(reportDir, "equity_curve.tsv"))
			if err != nil {
				return err
			}
			defer func() { _ = equityCurveTsv.Close() }()

			_ = equityCurveTsv.Write([]string{
				"time",
				"in_usd",
			})
			defer equityCurveTsv.Flush()

			kLineHandlers = append(kLineHandlers, func(k types.KLine, exSource *backtest.ExchangeDataSource) {
				if k.Interval != types.Interval1h {
					return
				}

				balances, err := exSource.Exchange.QueryAccountBalances(ctx)
				if err != nil {
					log.WithError(err).Errorf("query back-test account balance error")
				} else {
					assets := balances.Assets(exSource.Session.AllLastPrices(), k.EndTime.Time())
					_ = equityCurveTsv.Write([]string{
						k.EndTime.Time().Format(time.RFC1123),
						assets.InUSD().String(),
					})
				}
			})

			ordersTsv, err := tsv.NewWriterFile(filepath.Join(reportDir, "orders.tsv"))
			if err != nil {
				return err
			}
			defer func() { _ = ordersTsv.Close() }()
			_ = ordersTsv.Write(types.Order{}.CsvHeader())

			defer ordersTsv.Flush()
			for _, exSource := range exchangeSources {
				exSource.Session.UserDataStream.OnOrderUpdate(func(order types.Order) {
					if order.Status == types.OrderStatusFilled {
						for _, record := range order.CsvRecords() {
							_ = ordersTsv.Write(record)
						}
					}
				})
			}
		}

		runCtx, cancelRun := context.WithCancel(ctx)
		go func() {
			defer cancelRun()

			// Optimize back-test speed for single exchange source
			var numOfExchangeSources = len(exchangeSources)
			if numOfExchangeSources == 1 {
				exSource := exchangeSources[0]
				for k := range exSource.C {
					exSource.Exchange.ConsumeKLine(k)

					for _, h := range kLineHandlers {
						h(k, &exSource)
					}

				}

				if err := exSource.Exchange.CloseMarketData(); err != nil {
					log.WithError(err).Errorf("close market data error")
				}
				return
			}

		RunMultiExchangeData:
			for {
				for _, exK := range exchangeSources {
					k, more := <-exK.C
					if !more {
						if err := exK.Exchange.CloseMarketData(); err != nil {
							log.WithError(err).Errorf("close market data error")
							return
						}
						break RunMultiExchangeData
					}

					exK.Exchange.ConsumeKLine(k)

					for _, h := range kLineHandlers {
						h(k, &exK)
					}
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

		color.Green("BACK-TEST REPORT")
		color.Green("===============================================\n")
		color.Green("START TIME: %s\n", startTime.Format(time.RFC1123))
		color.Green("END TIME: %s\n", endTime.Format(time.RFC1123))

		// aggregate total balances
		initTotalBalances := types.BalanceMap{}
		finalTotalBalances := types.BalanceMap{}
		sessionNames := []string{}
		for _, session := range environ.Sessions() {
			sessionNames = append(sessionNames, session.Name)
			accountConfig := userConfig.Backtest.GetAccount(session.Name)
			initBalances := accountConfig.Balances.BalanceMap()
			initTotalBalances = initTotalBalances.Add(initBalances)

			finalBalances := session.GetAccount().Balances()
			finalTotalBalances = finalTotalBalances.Add(finalBalances)
		}
		color.Green("INITIAL TOTAL BALANCE: %v\n", initTotalBalances)
		color.Green("FINAL TOTAL BALANCE: %v\n", finalTotalBalances)

		summaryReport := &backtest.SummaryReport{
			StartTime:            startTime,
			EndTime:              endTime,
			Sessions:             sessionNames,
			InitialTotalBalances: initTotalBalances,
			FinalTotalBalances:   finalTotalBalances,
		}
		_ = summaryReport

		for _, session := range environ.Sessions() {
			backtestExchange, ok := session.Exchange.(*backtest.Exchange)
			if !ok {
				return fmt.Errorf("unexpected error, exchange instance is not a backtest exchange")
			}

			// per symbol report
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
					return fmt.Errorf("start price not found: %s, %s. run --sync first", symbol, exchangeName)
				}

				lastPrice, ok := session.LastPrice(symbol)
				if !ok {
					return fmt.Errorf("last price not found: %s, %s", symbol, exchangeName)
				}

				color.Green("%s %s PROFIT AND LOSS REPORT", strings.ToUpper(exchangeName), symbol)
				color.Green("===============================================")

				report := calculator.Calculate(symbol, trades.Trades, lastPrice)
				report.Print()

				accountConfig := userConfig.Backtest.GetAccount(exchangeName)
				initBalances := accountConfig.Balances.BalanceMap()
				finalBalances := session.GetAccount().Balances()

				if generatingReport {
					result := backtest.SessionSymbolReport{
						StartTime:       startTime,
						EndTime:         endTime,
						Symbol:          symbol,
						LastPrice:       lastPrice,
						StartPrice:      startPrice,
						PnLReport:       report,
						InitialBalances: initBalances,
						FinalBalances:   finalBalances,
						Manifests:       manifests,
					}

					if err := util.WriteJsonFile(filepath.Join(outputDirectory, symbol+".json"), &result); err != nil {
						return err
					}
				}

				initQuoteAsset := inQuoteAsset(initBalances, market, startPrice)
				finalQuoteAsset := inQuoteAsset(finalBalances, market, lastPrice)
				color.Green("INITIAL ASSET IN %s ~= %s %s (1 %s = %v)", market.QuoteCurrency, market.FormatQuantity(initQuoteAsset), market.QuoteCurrency, market.BaseCurrency, startPrice)
				color.Green("FINAL ASSET IN %s ~= %s %s (1 %s = %v)", market.QuoteCurrency, market.FormatQuantity(finalQuoteAsset), market.QuoteCurrency, market.BaseCurrency, lastPrice)

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

		if generatingReport && reportFileInSubDir {
			// append report index
			reportIndex, err := backtest.LoadReportIndex(outputDirectory)
			if err != nil {
				return err
			}

			reportIndex.Runs = append(reportIndex.Runs, backtest.Run{
				ID:     runID,
				Config: userConfig,
				Time:   time.Now(),
			})

			if err := backtest.WriteReportIndex(outputDirectory, reportIndex); err != nil {
				return err
			}
		}

		return nil
	},
}

func verify(userConfig *bbgo.Config, backtestService *service.BacktestService, sourceExchanges map[types.ExchangeName]types.Exchange, startTime time.Time, verboseCnt int) error {
	for _, sourceExchange := range sourceExchanges {
		err := backtestService.Verify(userConfig.Backtest.Symbols, startTime, time.Now(), sourceExchange, verboseCnt)
		if err != nil {
			return err
		}
	}
	return nil
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

func toExchangeSources(sessions map[string]*bbgo.ExchangeSession) (exchangeSources []backtest.ExchangeDataSource, err error) {
	for _, session := range sessions {
		exchange := session.Exchange.(*backtest.Exchange)
		exchange.InitMarketData()

		c, err := exchange.SubscribeMarketData(types.Interval1h, types.Interval1d)
		if err != nil {
			return exchangeSources, err
		}

		sessionCopy := session
		exchangeSources = append(exchangeSources, backtest.ExchangeDataSource{
			C:        c,
			Exchange: exchange,
			Session:  sessionCopy,
		})
	}
	return exchangeSources, nil
}

func sync(ctx context.Context, userConfig *bbgo.Config, backtestService *service.BacktestService, sourceExchanges map[types.ExchangeName]types.Exchange, syncFromTime time.Time) error {
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
	return nil
}

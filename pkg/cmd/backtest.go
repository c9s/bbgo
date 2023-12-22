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

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/data/tsv"
	"github.com/c9s/bbgo/pkg/util"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/backtest"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
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

		if userConfig.Backtest == nil {
			return errors.New("backtest config is not defined")
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var now = time.Now().Local()
		var startTime, endTime time.Time

		startTime = userConfig.Backtest.StartTime.Time().Local()

		// set default start time to the past 6 months
		// userConfig.Backtest.StartTime = now.AddDate(0, -6, 0).Format("2006-01-02")
		if userConfig.Backtest.EndTime != nil {
			endTime = userConfig.Backtest.EndTime.Time().Local()
		} else {
			endTime = now
		}

		// ensure that we're using local time
		startTime = startTime.Local()
		endTime = endTime.Local()

		log.Infof("starting backtest with startTime %s", startTime.Format(time.RFC3339))

		environ := bbgo.NewEnvironment()
		if err := bbgo.BootstrapBacktestEnvironment(ctx, environ); err != nil {
			return err
		}

		if environ.DatabaseService == nil {
			return errors.New("database service is not enabled, please check your environment variables DB_DRIVER and DB_DSN")
		}

		backtestService := &service.BacktestService{DB: environ.DatabaseService.DB}
		environ.BacktestService = backtestService
		bbgo.SetBackTesting(backtestService)

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

			publicExchange, err := exchange.NewPublic(exName)
			if err != nil {
				return err
			}
			sourceExchanges[exName] = publicExchange

			// Set exchange to use futures
			if userConfig.Sessions[exName.String()].Futures {
				futuresExchange, ok := publicExchange.(types.FuturesExchange)
				if !ok {
					return fmt.Errorf("exchange %s does not support futures", publicExchange.Name())
				}

				futuresExchange.UseFutures()
			}
		}

		var syncFromTime time.Time

		// user can override the sync from time if the option is given
		if len(syncFromDateStr) > 0 {
			syncFromTime, err = time.Parse(types.DateFormat, syncFromDateStr)
			if err != nil {
				return err
			}

			if syncFromTime.After(startTime) {
				return fmt.Errorf("sync-from time %s can not be latter than the backtest start time %s", syncFromTime, startTime)
			}

			syncFromTime = syncFromTime.Local()
		} else {
			// we need at least 1 month backward data for EMA and last prices
			syncFromTime = startTime.AddDate(0, -1, 0)
			log.Infof("adjusted sync start time %s to %s for backward market data", startTime, syncFromTime)
		}

		if wantSync {
			log.Infof("starting synchronization: %v", userConfig.Backtest.Symbols)
			if err := sync(ctx, userConfig, backtestService, sourceExchanges, syncFromTime, endTime); err != nil {
				return err
			}
			log.Info("synchronization done")

			if shouldVerify {
				err := verify(userConfig, backtestService, sourceExchanges, syncFromTime, endTime)
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

		if verboseCnt == 2 {
			log.SetLevel(log.DebugLevel)
		} else if verboseCnt > 0 {
			log.SetLevel(log.InfoLevel)
		} else {
			// default mode, disable strategy logging and order executor logging
			log.SetLevel(log.ErrorLevel)
		}

		environ.SetStartTime(startTime)

		// exchangeNameStr is the session name.
		for name, sourceExchange := range sourceExchanges {
			backtestExchange, err := backtest.NewExchange(sourceExchange.Name(), sourceExchange, backtestService, userConfig.Backtest)
			if err != nil {
				return errors.Wrap(err, "failed to create backtest exchange")
			}
			session := environ.AddExchange(name.String(), backtestExchange)
			exchangeFromConfig := userConfig.Sessions[name.String()]
			if exchangeFromConfig != nil {
				session.UseHeikinAshi = exchangeFromConfig.UseHeikinAshi
				session.Futures = exchangeFromConfig.Futures
			}
		}

		if err := environ.Init(ctx); err != nil {
			return err
		}

		for _, session := range environ.Sessions() {
			userDataStream := session.UserDataStream.(types.StandardStreamEmitter)
			backtestEx := session.Exchange.(*backtest.Exchange)
			backtestEx.MarketDataStream = session.MarketDataStream.(types.StandardStreamEmitter)
			backtestEx.BindUserData(userDataStream)
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

		allKLineIntervals, requiredInterval, backTestIntervals := backtest.CollectSubscriptionIntervals(environ)
		exchangeSources, err := backtest.InitializeExchangeSources(environ.Sessions(), startTime, endTime, requiredInterval, backTestIntervals...)
		if err != nil {
			return err
		}

		var kLineHandlers []func(k types.KLine, exSource *backtest.ExchangeDataSource)
		var manifests backtest.Manifests
		var runID = userConfig.GetSignature() + "_" + uuid.NewString()
		var reportDir = outputDirectory
		var sessionTradeStats = make(map[string]map[string]*types.TradeStats)

		// for each exchange session, iterate the positions and
		// allocate trade collector to calculate the tradeStats
		var tradeCollectorList []*core.TradeCollector
		for _, exSource := range exchangeSources {
			sessionName := exSource.Session.Name
			tradeStatsMap := make(map[string]*types.TradeStats)
			for usedSymbol := range exSource.Session.Positions() {
				market, _ := exSource.Session.Market(usedSymbol)
				position := types.NewPositionFromMarket(market)
				orderStore := core.NewOrderStore(usedSymbol)
				orderStore.AddOrderUpdate = true
				tradeCollector := core.NewTradeCollector(usedSymbol, position, orderStore)

				tradeStats := types.NewTradeStats(usedSymbol)
				tradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1d, startTime))
				tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
					if profit == nil {
						return
					}
					tradeStats.Add(profit)
				})
				tradeStatsMap[usedSymbol] = tradeStats

				orderStore.BindStream(exSource.Session.UserDataStream)
				tradeCollector.BindStream(exSource.Session.UserDataStream)
				tradeCollectorList = append(tradeCollectorList, tradeCollector)
			}
			sessionTradeStats[sessionName] = tradeStatsMap
		}

		kLineHandlers = append(kLineHandlers, func(k types.KLine, _ *backtest.ExchangeDataSource) {
			if k.Interval == types.Interval1d && k.Closed {
				for _, collector := range tradeCollectorList {
					collector.Process()
				}
			}
		})

		if generatingReport {
			if reportFileInSubDir {
				// reportDir = filepath.Join(reportDir, backtestSessionName)
				reportDir = filepath.Join(reportDir, runID)
			}
			if err := util.SafeMkdirAll(reportDir); err != nil {
				return err
			}

			startTimeStr := startTime.Format("20060102")
			endTimeStr := endTime.Format("20060102")
			kLineSubDir := strings.Join([]string{"klines", "_", startTimeStr, "-", endTimeStr}, "")
			kLineDataDir := filepath.Join(outputDirectory, "shared", kLineSubDir)
			if err := util.SafeMkdirAll(kLineDataDir); err != nil {
				return err
			}

			stateRecorder := backtest.NewStateRecorder(reportDir)
			err = trader.IterateStrategies(func(st bbgo.StrategyID) error {
				return stateRecorder.Scan(st.(backtest.Instance))
			})
			if err != nil {
				return err
			}

			manifests = stateRecorder.Manifests()
			manifests, err = rewriteManifestPaths(manifests, reportDir)
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
		for _, exK := range exchangeSources {
			exK.Callbacks = kLineHandlers
		}
		go func() {
			defer cancelRun()

			// Optimize back-test speed for single exchange source
			var numOfExchangeSources = len(exchangeSources)
			if numOfExchangeSources == 1 {
				exSource := exchangeSources[0]
				for k := range exSource.C {
					exSource.Exchange.ConsumeKLine(k, requiredInterval)
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

					exK.Exchange.ConsumeKLine(k, requiredInterval)
				}
			}
		}()

		cmdutil.WaitForSignal(runCtx, syscall.SIGINT, syscall.SIGTERM)

		log.Infof("shutting down trader...")

		gracefulShutdownPeriod := 30 * time.Second
		shtCtx, cancelShutdown := context.WithTimeout(bbgo.NewTodoContextWithExistingIsolation(ctx), gracefulShutdownPeriod)
		bbgo.Shutdown(shtCtx)
		cancelShutdown()

		// put the logger back to print the pnl
		log.SetLevel(log.InfoLevel)

		// aggregate total balances
		initTotalBalances := types.BalanceMap{}
		finalTotalBalances := types.BalanceMap{}
		var sessionNames []string
		for _, session := range environ.Sessions() {
			sessionNames = append(sessionNames, session.Name)
			accountConfig := userConfig.Backtest.GetAccount(session.Name)
			initBalances := accountConfig.Balances.BalanceMap()
			initTotalBalances = initTotalBalances.Add(initBalances)

			finalBalances := session.GetAccount().Balances()
			finalTotalBalances = finalTotalBalances.Add(finalBalances)
		}

		summaryReport := &backtest.SummaryReport{
			StartTime:            startTime,
			EndTime:              endTime,
			Sessions:             sessionNames,
			InitialTotalBalances: initTotalBalances,
			FinalTotalBalances:   finalTotalBalances,
			Manifests:            manifests,
			Symbols:              nil,
		}

		for interval := range allKLineIntervals {
			summaryReport.Intervals = append(summaryReport.Intervals, interval)
		}

		for _, session := range environ.Sessions() {
			for symbol, trades := range session.Trades {
				if len(trades.Trades) == 0 {
					log.Warnf("session has no %s trades", symbol)
					continue
				}

				tradeState := sessionTradeStats[session.Name][symbol]
				profitFactor := tradeState.ProfitFactor
				winningRatio := tradeState.WinningRatio
				intervalProfits := tradeState.IntervalProfits[types.Interval1d]

				symbolReport, err := createSymbolReport(userConfig, session, symbol, trades.Copy(), intervalProfits, profitFactor, winningRatio)
				if err != nil {
					return err
				}

				summaryReport.Symbols = append(summaryReport.Symbols, symbol)
				summaryReport.SymbolReports = append(summaryReport.SymbolReports, *symbolReport)
				summaryReport.TotalProfit = symbolReport.PnL.Profit
				summaryReport.TotalUnrealizedProfit = symbolReport.PnL.UnrealizedProfit
				summaryReport.InitialEquityValue = summaryReport.InitialEquityValue.Add(symbolReport.InitialEquityValue())
				summaryReport.FinalEquityValue = summaryReport.FinalEquityValue.Add(symbolReport.FinalEquityValue())
				summaryReport.TotalGrossProfit.Add(symbolReport.PnL.GrossProfit)
				summaryReport.TotalGrossLoss.Add(symbolReport.PnL.GrossLoss)

				// write report to a file
				if generatingReport {
					reportFileName := fmt.Sprintf("symbol_report_%s_%s.json", session.Name, symbol)
					if err := util.WriteJsonFile(filepath.Join(reportDir, reportFileName), &symbolReport); err != nil {
						return err
					}
				}
			}
		}

		if generatingReport {
			summaryReportFile := filepath.Join(reportDir, "summary.json")

			// output summary report filepath to stdout, so that our optimizer can read from it
			fmt.Println(summaryReportFile)

			if err := util.WriteJsonFile(summaryReportFile, summaryReport); err != nil {
				return errors.Wrapf(err, "can not write summary report json file: %s", summaryReportFile)
			}

			configJsonFile := filepath.Join(reportDir, "config.json")
			if err := util.WriteJsonFile(configJsonFile, userConfig); err != nil {
				return errors.Wrapf(err, "can not write config json file: %s", configJsonFile)
			}

			// append report index
			if reportFileInSubDir {
				if err := backtest.AddReportIndexRun(outputDirectory, backtest.Run{
					ID:     runID,
					Config: userConfig,
					Time:   time.Now(),
				}); err != nil {
					return err
				}
			}
		} else {
			color.Green("BACK-TEST REPORT")
			color.Green("===============================================\n")
			color.Green("START TIME: %s\n", startTime.Format(time.RFC1123))
			color.Green("END TIME: %s\n", endTime.Format(time.RFC1123))
			color.Green("INITIAL TOTAL BALANCE: %v\n", initTotalBalances)
			color.Green("FINAL TOTAL BALANCE: %v\n", finalTotalBalances)
			for _, symbolReport := range summaryReport.SymbolReports {
				symbolReport.Print(wantBaseAssetBaseline)
			}
		}

		return nil
	},
}

func createSymbolReport(
	userConfig *bbgo.Config, session *bbgo.ExchangeSession, symbol string, trades []types.Trade,
	intervalProfit *types.IntervalProfitCollector,
	profitFactor, winningRatio fixedpoint.Value,
) (
	*backtest.SessionSymbolReport,
	error,
) {
	backtestExchange, ok := session.Exchange.(*backtest.Exchange)
	if !ok {
		return nil, fmt.Errorf("unexpected error, exchange instance is not a backtest exchange")
	}

	market, ok := session.Market(symbol)
	if !ok {
		return nil, fmt.Errorf("market not found: %s, %s", symbol, session.Exchange.Name())
	}

	startPrice, ok := session.StartPrice(symbol)
	if !ok {
		return nil, fmt.Errorf("start price not found: %s, %s. run --sync first", symbol, session.Exchange.Name())
	}

	lastPrice, ok := session.LastPrice(symbol)
	if !ok {
		return nil, fmt.Errorf("last price not found: %s, %s", symbol, session.Exchange.Name())
	}

	calculator := &pnl.AverageCostCalculator{
		TradingFeeCurrency: backtestExchange.PlatformFeeCurrency(),
		Market:             market,
	}

	sharpeRatio := fixedpoint.NewFromFloat(intervalProfit.GetSharpe())
	sortinoRatio := fixedpoint.NewFromFloat(intervalProfit.GetSortino())

	report := calculator.Calculate(symbol, trades, lastPrice)
	accountConfig := userConfig.Backtest.GetAccount(session.Exchange.Name().String())
	initBalances := accountConfig.Balances.BalanceMap()
	finalBalances := session.GetAccount().Balances()
	symbolReport := backtest.SessionSymbolReport{
		Exchange:        session.Exchange.Name(),
		Symbol:          symbol,
		Market:          market,
		LastPrice:       lastPrice,
		StartPrice:      startPrice,
		PnL:             report,
		InitialBalances: initBalances,
		FinalBalances:   finalBalances,
		// Manifests:       manifests,
		Sharpe:       sharpeRatio,
		Sortino:      sortinoRatio,
		ProfitFactor: profitFactor,
		WinningRatio: winningRatio,
	}

	for _, s := range session.Subscriptions {
		symbolReport.Subscriptions = append(symbolReport.Subscriptions, s)
	}

	sessionKLineIntervals := map[types.Interval]struct{}{}
	for _, sub := range session.Subscriptions {
		if sub.Channel == types.KLineChannel {
			sessionKLineIntervals[sub.Options.Interval] = struct{}{}
		}
	}

	for interval := range sessionKLineIntervals {
		symbolReport.Intervals = append(symbolReport.Intervals, interval)
	}

	return &symbolReport, nil
}

func verify(
	userConfig *bbgo.Config, backtestService *service.BacktestService,
	sourceExchanges map[types.ExchangeName]types.Exchange, startTime, endTime time.Time,
) error {
	for _, sourceExchange := range sourceExchanges {
		err := backtestService.Verify(sourceExchange, userConfig.Backtest.Symbols, startTime, endTime)
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

func getExchangeIntervals(ex types.Exchange) types.IntervalMap {
	exCustom, ok := ex.(types.CustomIntervalProvider)
	if ok {
		return exCustom.SupportedInterval()
	}
	return types.SupportedIntervals
}

func sync(
	ctx context.Context, userConfig *bbgo.Config, backtestService *service.BacktestService,
	sourceExchanges map[types.ExchangeName]types.Exchange, syncFrom, syncTo time.Time,
) error {
	for _, symbol := range userConfig.Backtest.Symbols {
		for _, sourceExchange := range sourceExchanges {
			var supportIntervals = getExchangeIntervals(sourceExchange)

			if !userConfig.Backtest.SyncSecKLines {
				delete(supportIntervals, types.Interval1s)
			}

			// sort intervals
			var intervals = supportIntervals.Slice()
			intervals.Sort()

			for _, interval := range intervals {
				if err := backtestService.Sync(ctx, sourceExchange, symbol, interval, syncFrom, syncTo); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func rewriteManifestPaths(manifests backtest.Manifests, basePath string) (backtest.Manifests, error) {
	var filterManifests = backtest.Manifests{}
	for k, m := range manifests {
		p, err := filepath.Rel(basePath, m)
		if err != nil {
			return nil, err
		}
		filterManifests[k] = p
	}
	return filterManifests, nil
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/accounting"
	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	PnLCmd.Flags().String("session", "", "target exchange")
	PnLCmd.Flags().String("symbol", "", "trading symbol")
	PnLCmd.Flags().Bool("include-transfer", false, "convert transfer records into trades")
	PnLCmd.Flags().Int("limit", 500, "number of trades")
	RootCmd.AddCommand(PnLCmd)
}

var PnLCmd = &cobra.Command{
	Use:          "pnl",
	Short:        "pnl calculator",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		if len(configFile) == 0 {
			return errors.New("--config option is required")
		}

		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			return err
		}

		userConfig, err := bbgo.Load(configFile, false)
		if err != nil {
			return err
		}

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		if len(symbol) == 0 {
			return errors.New("--symbol [SYMBOL] is required")
		}

		limit, err := cmd.Flags().GetInt("limit")
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()

		if err := environ.ConfigureDatabase(ctx); err != nil {
			return err
		}

		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		session, ok := environ.Session(sessionName)
		if !ok {
			return fmt.Errorf("session %s not found", sessionName)
		}

		if err := environ.SyncSession(ctx, session); err != nil {
			return err
		}

		if err = environ.Init(ctx); err != nil {
			return err
		}

		exchange := session.Exchange

		market, ok := session.Market(symbol)
		if !ok {
			return fmt.Errorf("market config %s not found", symbol)
		}

		since := time.Now().AddDate(-1, 0, 0)
		until := time.Now()

		includeTransfer, err := cmd.Flags().GetBool("include-transfer")
		if err != nil {
			return err
		}

		if includeTransfer {
			transferService, ok := exchange.(types.ExchangeTransferService)
			if !ok {
				return fmt.Errorf("session exchange %s does not implement transfer service", sessionName)
			}

			deposits, err := transferService.QueryDepositHistory(ctx, market.BaseCurrency, since, until)
			if err != nil {
				return err
			}
			_ = deposits

			withdrawals, err := transferService.QueryWithdrawHistory(ctx, market.BaseCurrency, since, until)
			if err != nil {
				return err
			}
			_ = withdrawals

			// we need the backtest klines for the daily prices
			backtestService := &service.BacktestService{DB: environ.DatabaseService.DB}
			if err := backtestService.SyncKLineByInterval(ctx, exchange, symbol, types.Interval1d, since, until); err != nil {
				return err
			}
		}

		var trades []types.Trade
		tradingFeeCurrency := exchange.PlatformFeeCurrency()
		if strings.HasPrefix(symbol, tradingFeeCurrency) {
			log.Infof("loading all trading fee currency related trades: %s", symbol)
			trades, err = environ.TradeService.QueryForTradingFeeCurrency(exchange.Name(), symbol, tradingFeeCurrency)
		} else {
			trades, err = environ.TradeService.Query(service.QueryTradesOptions{
				Exchange: exchange.Name(),
				Symbol:   symbol,
				Limit:    limit,
			})
		}

		if err != nil {
			return err
		}

		log.Infof("%d trades loaded", len(trades))

		stockManager := &accounting.StockDistribution{
			Symbol:             symbol,
			TradingFeeCurrency: tradingFeeCurrency,
		}

		checkpoints, err := stockManager.AddTrades(trades)
		if err != nil {
			return err
		}

		log.Infof("found checkpoints: %+v", checkpoints)
		log.Infof("stock: %v", stockManager.Stocks.Quantity())

		tickers, err := exchange.QueryTickers(ctx, symbol)

		if err != nil {
			return err
		}

		currentTick, ok := tickers[symbol]

		if !ok {
			return errors.New("no ticker data for current price")
		}

		currentPrice := currentTick.Last

		calculator := &pnl.AverageCostCalculator{
			TradingFeeCurrency: tradingFeeCurrency,
		}

		report := calculator.Calculate(symbol, trades, currentPrice)
		report.Print()
		return nil
	},
}

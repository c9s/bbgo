package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/accounting"
	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	PnLCmd.Flags().String("exchange", "", "target exchange")
	PnLCmd.Flags().String("symbol", "BTCUSDT", "trading symbol")
	PnLCmd.Flags().String("since", "", "pnl since time")
	RootCmd.AddCommand(PnLCmd)
}

var PnLCmd = &cobra.Command{
	Use:          "pnl",
	Short:        "pnl calculator",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		exchangeNameStr, err := cmd.Flags().GetString("exchange")
		if err != nil {
			return err
		}

		exchangeName, err := types.ValidExchangeName(exchangeNameStr)
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		exchange, err := cmdutil.NewExchange(exchangeName)
		if err != nil {
			return err
		}

		db, err := cmdutil.ConnectMySQL()
		if err != nil {
			return err
		}

		since, err := cmd.Flags().GetString("since")
		if err != nil {
			return err
		}

		var startTime = time.Now().AddDate(-2, 0, 0)
		if len(since) > 0 {
			loc, err := time.LoadLocation("Asia/Taipei")
			if err != nil {
				return err
			}

			startTime, err = time.ParseInLocation("2006-01-02", since, loc)
			if err != nil {
				return err
			}
		}

		tradeService := &service.TradeService{DB: db}
		tradeSync := &service.TradeSync{Service: tradeService}

		logrus.Info("syncing trades from exchange...")
		if err := tradeSync.Sync(ctx, exchange, symbol, startTime); err != nil {
			return err
		}

		var trades []types.Trade
		tradingFeeCurrency := exchange.PlatformFeeCurrency()
		if strings.HasPrefix(symbol, tradingFeeCurrency) {
			logrus.Infof("loading all trading fee currency related trades: %s", symbol)
			trades, err = tradeService.QueryForTradingFeeCurrency(symbol, tradingFeeCurrency)
		} else {
			trades, err = tradeService.Query(symbol)
		}

		if err != nil {
			return err
		}

		logrus.Infof("%d trades loaded", len(trades))

		stockManager := &accounting.StockDistribution{
			Symbol:             symbol,
			TradingFeeCurrency: tradingFeeCurrency,
		}

		checkpoints, err := stockManager.AddTrades(trades)
		if err != nil {
			return err
		}

		logrus.Infof("found checkpoints: %+v", checkpoints)
		logrus.Infof("stock: %f", stockManager.Stocks.Quantity())

		currentPrice, err := exchange.QueryAveragePrice(ctx, symbol)

		calculator := &pnl.AverageCostCalculator{
			TradingFeeCurrency: tradingFeeCurrency,
		}

		report := calculator.Calculate(symbol, trades, currentPrice)
		report.Print()
		return nil
	},
}

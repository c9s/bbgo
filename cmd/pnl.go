package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/accounting"
	"github.com/c9s/bbgo/bbgo"
	binance2 "github.com/c9s/bbgo/exchange/binance"
	"github.com/c9s/bbgo/service"
	"github.com/c9s/bbgo/types"
)

func init() {
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

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		binanceKey := viper.GetString("binance-api-key")
		binanceSecret := viper.GetString("binance-api-secret")
		binanceExchange := binance2.New(binanceKey, binanceSecret)

		mysqlURL := viper.GetString("mysql-url")
		mysqlURL = fmt.Sprintf("%s?parseTime=true", mysqlURL)
		db, err := sqlx.Connect("mysql", mysqlURL)
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

		logrus.Info("syncing trades...")
		if err := tradeSync.Sync(ctx, binanceExchange, symbol, startTime); err != nil {
			return err
		}

		var trades []types.Trade
		tradingFeeCurrency := binanceExchange.PlatformFeeCurrency()
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

		stockManager := &bbgo.StockDistribution{
			Symbol:             symbol,
			TradingFeeCurrency: tradingFeeCurrency,
		}

		checkpoints, err := stockManager.AddTrades(trades)
		if err != nil {
			return err
		}

		logrus.Infof("found checkpoints: %+v", checkpoints)
		logrus.Infof("stock: %f", stockManager.Stocks.Quantity())

		currentPrice, err := binanceExchange.QueryAveragePrice(ctx, symbol)

		calculator := &accounting.ProfitAndLossCalculator{
			TradingFeeCurrency: tradingFeeCurrency,
			Symbol:             symbol,
			StartTime:          startTime,
			CurrentPrice:       currentPrice,
			Trades:             trades,
		}
		report := calculator.Calculate()
		report.Print()
		return nil
	},
}

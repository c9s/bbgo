package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/accounting"
	"github.com/c9s/bbgo/bbgo"
	"github.com/c9s/bbgo/exchange/binance"
	"github.com/c9s/bbgo/exchange/max"
	"github.com/c9s/bbgo/service"
	"github.com/c9s/bbgo/types"
)

func init() {
	PnLCmd.Flags().String("exchange", "", "target exchange")
	PnLCmd.Flags().String("symbol", "BTCUSDT", "trading symbol")
	PnLCmd.Flags().String("since", "", "pnl since time")
	RootCmd.AddCommand(PnLCmd)
}

func connectMysql() (*sqlx.DB, error) {
	mysqlURL := viper.GetString("mysql-url")
	mysqlURL = fmt.Sprintf("%s?parseTime=true", mysqlURL)
	return sqlx.Connect("mysql", mysqlURL)
}

func newExchangeFromViper(n types.ExchangeName) types.Exchange {
	switch n {

	case types.ExchangeBinance:
		key := viper.GetString("binance-api-key")
		secret := viper.GetString("binance-api-secret")
		return binance.New(key, secret)

	case types.ExchangeMax:
		key := viper.GetString("max-api-key")
		secret := viper.GetString("max-api-secret")
		return max.New(key, secret)

	}

	return nil
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

		exchange := newExchangeFromViper(exchangeName)

		db, err := connectMysql()
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

		log.Info("syncing trades from exchange...")
		if err := tradeSync.Sync(ctx, exchange, symbol, startTime); err != nil {
			return err
		}

		var trades []types.Trade
		tradingFeeCurrency := exchange.PlatformFeeCurrency()
		if strings.HasPrefix(symbol, tradingFeeCurrency) {
			log.Infof("loading all trading fee currency related trades: %s", symbol)
			trades, err = tradeService.QueryForTradingFeeCurrency(symbol, tradingFeeCurrency)
		} else {
			trades, err = tradeService.Query(symbol)
		}

		if err != nil {
			return err
		}

		log.Infof("%d trades loaded", len(trades))

		stockManager := &bbgo.StockDistribution{
			Symbol:             symbol,
			TradingFeeCurrency: tradingFeeCurrency,
		}

		checkpoints, err := stockManager.AddTrades(trades)
		if err != nil {
			return err
		}

		log.Infof("found checkpoints: %+v", checkpoints)
		log.Infof("stock: %f", stockManager.Stocks.Quantity())

		currentPrice, err := exchange.QueryAveragePrice(ctx, symbol)

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

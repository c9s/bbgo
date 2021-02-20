package cmd

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/accounting"
	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	PnLCmd.Flags().String("exchange", "", "target exchange")
	PnLCmd.Flags().String("symbol", "BTCUSDT", "trading symbol")
	PnLCmd.Flags().Int("limit", 500, "number of trades")
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

		limit, err := cmd.Flags().GetInt("limit")
		if err != nil {
			return err
		}

		exchange, err := cmdutil.NewExchange(exchangeName)
		if err != nil {
			return err
		}


		environ := bbgo.NewEnvironment()
		if err := environ.ConfigureDatabase(ctx) ; err != nil {
			return err
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
				Limit:	  limit,
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
		log.Infof("stock: %f", stockManager.Stocks.Quantity())

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

package buyandhold

import (
	"context"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	// flags for strategies
	Cmd.Flags().String("exchange", "binance", "target exchange")
	Cmd.Flags().String("symbol", "BTCUSDT", "trading symbol")
	Cmd.Flags().String("interval", "1h", "kline interval for price change detection")
	Cmd.Flags().Float64("base-quantity", 0.1, "base quantity of the order")
}

var Cmd = &cobra.Command{
	Use:   "buyandhold",
	Short: "buy and hold",
	Long:  "buy and hold strategy",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

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

		interval, err := cmd.Flags().GetString("interval")
		if err != nil {
			return err
		}

		baseQuantity, err := cmd.Flags().GetFloat64("base-quantity")
		if err != nil {
			return err
		}

		db, err := cmdutil.ConnectMySQL()
		if err != nil {
			return err
		}

		environ := bbgo.NewDefaultEnvironment()
		environ.SyncTrades(db)
		trader := bbgo.NewTrader(environ)
		trader.AttachStrategyOn(string(exchangeName), New(symbol, interval, baseQuantity))

		err = trader.Run(ctx)
		if err != nil {
			return err
		}

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return err
	},
}

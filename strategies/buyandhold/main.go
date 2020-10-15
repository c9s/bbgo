package main

import (
	"context"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/strategy/buyandhold"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	rootCmd.PersistentFlags().String("binance-api-key", "", "binance api key")
	rootCmd.PersistentFlags().String("binance-api-secret", "", "binance api secret")
	rootCmd.PersistentFlags().String("max-api-key", "", "max api key")
	rootCmd.PersistentFlags().String("max-api-secret", "", "max api secret")

	rootCmd.Flags().String("exchange", "binance", "target exchange")
	rootCmd.Flags().String("symbol", "BTCUSDT", "trading symbol")
	rootCmd.Flags().String("interval", "1h", "kline interval for price change detection")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Enable environment variable binding, the env vars are not overloaded yet.
	viper.AutomaticEnv()

	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.WithError(err).Errorf("failed to bind persistent flags. please check the flag settings.")
	}
}

var rootCmd = &cobra.Command{
	Use:   "buyandhold",
	Short: "buy and hold",
	Long:  "hold trader",

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

		exchange, err := cmdutil.NewExchange(exchangeName)
		if err != nil {
			return err
		}

		db, err := cmdutil.ConnectMySQL()
		if err != nil {
			return err
		}

		sessionID := "main"
		environ := bbgo.NewEnvironment(db)
		environ.AddExchange(sessionID, exchange)

		trader := bbgo.NewTrader(environ)
		trader.AttachStrategy(sessionID, buyandhold.New(symbol, interval, 0.1))
		// trader.AttachCrossExchangeStrategy(...)
		err = trader.Run(ctx)

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return err
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.WithError(err).Fatalf("cannot execute command")
	}
}

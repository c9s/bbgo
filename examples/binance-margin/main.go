package main

import (
	"context"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/exchange/binance"
)

func init() {
	rootCmd.PersistentFlags().String("binance-api-key", "", "binance api key")
	rootCmd.PersistentFlags().String("binance-api-secret", "", "binance api secret")
	rootCmd.PersistentFlags().String("symbol", "BNBUSDT", "symbol")
}

var rootCmd = &cobra.Command{
	Use:   "binance-margin",
	Short: "binance margin",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		key, secret := viper.GetString("binance-api-key"), viper.GetString("binance-api-secret")
		if len(key) == 0 || len(secret) == 0 {
			return errors.New("empty key or secret")
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		var exchange = binance.New(key, secret)

		exchange.UseIsolatedMargin(symbol)

		marginAccount, err := exchange.QueryMarginAccount(ctx)
		if err != nil {
			return err
		}

		log.Infof("margin account: %+v", marginAccount)

		isolatedMarginAccount, err := exchange.QueryIsolatedMarginAccount(ctx)
		if err != nil {
			return err
		}

		log.Infof("isolated margin account: %+v", isolatedMarginAccount)

		stream := exchange.NewStream()

		log.Info("connecting websocket...")
		if err := stream.Connect(ctx); err != nil {
			log.Fatal(err)
		}

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

func main() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.WithError(err).Error("bind pflags error")
	}

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		log.WithError(err).Error("cmd error")
	}
}

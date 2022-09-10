package main

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	rootCmd.PersistentFlags().String("binance-api-key", "", "binance api key")
	rootCmd.PersistentFlags().String("binance-api-secret", "", "binance api secret")
	rootCmd.PersistentFlags().String("symbol", "BNBUSDT", "symbol")
	rootCmd.PersistentFlags().Float64("price", 20.0, "order price")
	rootCmd.PersistentFlags().Float64("quantity", 10.0, "order quantity")
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

		price, err := cmd.Flags().GetFloat64("price")
		if err != nil {
			return err
		}

		quantity, err := cmd.Flags().GetFloat64("quantity")
		if err != nil {
			return err
		}

		var exchange = binance.New(key, secret)

		exchange.UseIsolatedMargin(symbol)

		markets, err := exchange.QueryMarkets(ctx)
		if err != nil {
			return err
		}

		market, ok := markets[symbol]
		if !ok {
			return fmt.Errorf("market %s is not defined", symbol)
		}

		marginAccount, err := exchange.QueryCrossMarginAccount(ctx)
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

		time.Sleep(time.Second)

		createdOrder, err := exchange.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:           symbol,
			Market:           market,
			Side:             types.SideTypeBuy,
			Type:             types.OrderTypeLimit,
			Price:            fixedpoint.NewFromFloat(price),
			Quantity:         fixedpoint.NewFromFloat(quantity),
			MarginSideEffect: types.SideEffectTypeMarginBuy,
			TimeInForce:      "GTC",
		})
		if err != nil {
			return err
		}

		log.Info(createdOrder)

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

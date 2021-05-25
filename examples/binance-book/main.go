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
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	rootCmd.PersistentFlags().String("binance-api-key", "", "binance api key")
	rootCmd.PersistentFlags().String("binance-api-secret", "", "binance api secret")
	rootCmd.PersistentFlags().String("symbol", "BNBUSDT", "symbol")
}

var rootCmd = &cobra.Command{
	Use:   "binance-book",
	Short: "binance book",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		symbol := viper.GetString("symbol")
		if len(symbol) == 0 {
			return errors.New("empty symbol")
		}

		key, secret := viper.GetString("binance-api-key"), viper.GetString("binance-api-secret")
		if len(key) == 0 || len(secret) == 0 {
			return errors.New("empty key or secret")
		}

		var exchange = binance.New(key, secret)

		stream := exchange.NewStream()
		stream.SetPublicOnly()
		stream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})

		streamBook := types.NewStreamBook(symbol)
		streamBook.BindStream(stream)

		go func() {
			for {
				select {

				case <-ctx.Done():
					return

				case <-streamBook.C:
					book := streamBook.Copy()

					if valid, err := book.IsValid(); !valid {
						log.Errorf("order book is invalid, error: %v", err)
						continue
					}

					bestBid, hasBid := book.BestBid()
					bestAsk, hasAsk := book.BestAsk()
					if hasBid && hasAsk {
						log.Infof("================================")
						log.Infof("best ask %f % -12f",
							bestAsk.Price.Float64(),
							bestAsk.Volume.Float64(),
						)
						log.Infof("best bid %f % -12f",
							bestBid.Price.Float64(),
							bestBid.Volume.Float64(),
						)
					}
				}

			}
		}()

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

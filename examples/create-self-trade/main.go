package main

import (
	"context"
	"fmt"
	"math"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	rootCmd.PersistentFlags().String("exchange", "binance", "exchange name")
	rootCmd.PersistentFlags().String("symbol", "SANDUSDT", "symbol")
}

var rootCmd = &cobra.Command{
	Use:   "create-self-trade",
	Short: "this program creates the self trade by getting the market ticker",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := godotenv.Load(".env.local"); err != nil {
			log.Fatal(err)
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		exchangeNameStr, err := cmd.Flags().GetString("exchange")
		if err != nil {
			return err
		}

		exchangeName, err := types.ValidExchangeName(exchangeNameStr)
		if err != nil {
			return err
		}

		exchange, err := cmdutil.NewExchange(exchangeName)
		if err != nil {
			return err
		}

		markets, err := exchange.QueryMarkets(ctx)
		if err != nil {
			return err
		}

		market, ok := markets[symbol]
		if !ok {
			return fmt.Errorf("market %s is not defined", symbol)
		}

		stream := exchange.NewStream()
		stream.OnTradeUpdate(func(trade types.Trade) {
			log.Infof("trade: %+v", trade)
		})

		log.Info("connecting websocket...")
		if err := stream.Connect(ctx); err != nil {
			log.Fatal(err)
		}

		time.Sleep(time.Second)

		ticker, err := exchange.QueryTicker(ctx, symbol)
		if err != nil {
			log.Fatal(err)
		}

		price := ticker.Buy + market.TickSize

		if int64(ticker.Sell*1e8) == int64(price*1e8) {
			log.Fatal("zero spread, can not continue")
		}

		quantity := math.Max(market.MinNotional/price, market.MinQuantity) * 1.1

		log.Infof("submiting order using quantity %f at price %f", quantity, price)

		createdOrders, err := exchange.SubmitOrders(ctx, []types.SubmitOrder{
			{
				Symbol:      symbol,
				Market:      market,
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Price:       price,
				Quantity:    quantity,
				TimeInForce: "GTC",
			},
			{
				Symbol:      symbol,
				Market:      market,
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeLimit,
				Price:       price,
				Quantity:    quantity,
				TimeInForce: "GTC",
			},
		}...)

		if err != nil {
			return err
		}

		log.Info(createdOrders)

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

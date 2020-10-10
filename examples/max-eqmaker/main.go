package main

import (
	"context"
	"math"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/cmd/cmdutil"
	"github.com/c9s/bbgo/exchange/max"
	maxapi "github.com/c9s/bbgo/exchange/max/maxapi"
	"github.com/c9s/bbgo/fixedpoint"
	"github.com/c9s/bbgo/types"
	"github.com/c9s/bbgo/util"
)

func init() {
	rootCmd.PersistentFlags().String("max-api-key", "", "max api key")
	rootCmd.PersistentFlags().String("max-api-secret", "", "max api secret")
	rootCmd.PersistentFlags().String("symbol", "maxusdt", "symbol")

	rootCmd.Flags().String("side", "buy", "side")
	rootCmd.Flags().Int("num-orders", 5, "number of orders for one side")
	rootCmd.Flags().Float64("behind-volume", 1000.0, "behind volume depth")
	rootCmd.Flags().Float64("base-quantity", 100.0, "base quantity")
	rootCmd.Flags().Float64("price-tick", 0.02, "price tick")
	rootCmd.Flags().Float64("buy-sell-ratio", 1.0, "price tick")
}

var rootCmd = &cobra.Command{
	Use:   "trade",
	Short: "start trader",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		symbol := viper.GetString("symbol")
		if len(symbol) == 0 {
			return errors.New("empty symbol")
		}

		key, secret := viper.GetString("max-api-key"), viper.GetString("max-api-secret")
		if len(key) == 0 || len(secret) == 0 {
			return errors.New("empty key or secret")
		}

		side, err := cmd.Flags().GetString("side")
		if err != nil {
			return err
		}

		iv, err := cmd.Flags().GetInt("num-orders")
		if err != nil {
			return err
		}
		var numOrders = iv

		fv, err := cmd.Flags().GetFloat64("base-quantity")
		if err != nil {
			return err
		}
		var baseQuantity = fixedpoint.NewFromFloat(fv)

		fv, err = cmd.Flags().GetFloat64("price-tick")
		if err != nil {
			return err
		}
		var priceTick = fixedpoint.NewFromFloat(fv)

		fv, err = cmd.Flags().GetFloat64("behind-volume")
		if err != nil {
			return err
		}

		var behindVolume = fixedpoint.NewFromFloat(fv)

		buySellRatio, err := cmd.Flags().GetFloat64("buy-sell-ratio")
		if err != nil {
			return err
		}

		maxRest := maxapi.NewRestClient(maxapi.ProductionAPIURL)
		maxRest.Auth(key, secret)

		stream := max.NewStream(key, secret)
		stream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})

		streambook := types.NewStreamBook(symbol)
		streambook.BindStream(stream)

		cancelSideOrders := func(symbol string, side string) {
			if err := maxRest.OrderService.CancelAll(side, symbol); err != nil {
				log.WithError(err).Error("cancel all error")
			}

			streambook.C.Drain(2*time.Second, 5*time.Second)
		}

		updateSideOrders := func(symbol string, side string, baseQuantity fixedpoint.Value) {
			book := streambook.Copy()

			var pvs types.PriceVolumeSlice

			switch side {
			case "buy":
				pvs = book.Bids
			case "sell":
				pvs = book.Asks
			}

			if pvs == nil || len(pvs) == 0 {
				log.Warn("empty bids or asks")
				return
			}

			index := pvs.IndexByVolumeDepth(behindVolume)
			if index == -1 {
				// do not place orders
				log.Warn("depth is not enough")
				return
			}

			var price = pvs[index].Price
			var orders = generateOrders(symbol, side, price, priceTick, baseQuantity, numOrders)
			if len(orders) == 0 {
				log.Warn("empty orders")
				return
			}
			log.Infof("submitting %d orders", len(orders))

			retOrders, err := maxRest.OrderService.CreateMulti(symbol, orders)
			if err != nil {
				log.WithError(err).Error("create multi error")
			}
			_ = retOrders

			streambook.C.Drain(2*time.Second, 5*time.Second)
		}

		update := func() {
			switch side {
			case "both":
				cancelSideOrders(symbol, "buy")
				updateSideOrders(symbol, "buy", baseQuantity.MulFloat64(buySellRatio))

				cancelSideOrders(symbol, "sell")
				updateSideOrders(symbol, "sell", baseQuantity.MulFloat64(1.0/buySellRatio))

			default:
				cancelSideOrders(symbol, side)
				updateSideOrders(symbol, side, baseQuantity)
			}
		}

		go func() {
			ticker := time.NewTicker(1 * time.Minute)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return

				case <-streambook.C:
					streambook.C.Drain(2*time.Second, 5*time.Second)
					update()

				case <-ticker.C:
					update()
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

func generateOrders(symbol, side string, price, priceTick, baseVolume fixedpoint.Value, numOrders int) (orders []maxapi.Order) {
	var expBase = fixedpoint.NewFromFloat(0.0)

	switch side {
	case "buy":
		if priceTick > 0 {
			priceTick = -priceTick
		}
	case "sell":
		if priceTick < 0 {
			priceTick = -priceTick
		}
	}

	for i := 0; i < numOrders; i++ {
		volume := math.Exp(expBase.Float64()) * baseVolume.Float64()

		// skip order less than 10usd
		if volume*price.Float64() < 10.0 {
			log.Warnf("amount too small (< 10usd). price=%f volume=%f amount=%f", price, volume, volume*price.Float64())
			continue
		}

		orders = append(orders, maxapi.Order{
			Side:      side,
			OrderType: string(maxapi.OrderTypeLimit),
			Market:    symbol,
			Price:     util.FormatFloat(price.Float64(), 3),
			Volume:    util.FormatFloat(volume, 2),
			// GroupID:         0,
		})

		log.Infof("%s order: %.2f @ %.3f", side, volume, price.Float64())

		if len(orders) >= numOrders {
			break
		}

		price = price + priceTick
		declog := math.Log10(math.Abs(priceTick.Float64()))
		expBase += fixedpoint.NewFromFloat(math.Pow10(-int(declog)) * math.Abs(priceTick.Float64()))
		log.Infof("expBase: %f", expBase.Float64())
	}

	return orders
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

package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.PersistentFlags().String("okex-api-key", "", "okex api key")
	rootCmd.PersistentFlags().String("okex-api-secret", "", "okex api secret")
	rootCmd.PersistentFlags().String("okex-api-passphrase", "", "okex api secret")
	rootCmd.PersistentFlags().String("symbol", "BNBUSDT", "symbol")
}

var rootCmd = &cobra.Command{
	Use:   "okex-book",
	Short: "okex book",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		symbol := viper.GetString("symbol")
		if len(symbol) == 0 {
			return errors.New("empty symbol")
		}

		key, secret, passphrase := viper.GetString("okex-api-key"),
			viper.GetString("okex-api-secret"),
			viper.GetString("okex-api-passphrase")
		if len(key) == 0 || len(secret) == 0 {
			return errors.New("empty key, secret or passphrase")
		}

		client := okexapi.NewClient()

		client.Auth(key, secret, passphrase)

		instruments, err := client.NewGetInstrumentsInfoRequest().
			InstType("SPOT").Do(ctx)
		if err != nil {
			return err
		}

		log.Infof("instruments: %+v", instruments)

		fundingRate, err := client.NewGetFundingRate().InstrumentID("BTC-USDT-SWAP").Do(ctx)
		if err != nil {
			return err
		}
		log.Infof("funding rate: %+v", fundingRate)

		log.Infof("ACCOUNT BALANCES:")
		account, err := client.NewGetAccountBalanceRequest().Do(ctx)
		if err != nil {
			return err
		}

		log.Infof("%+v", account)

		log.Infof("ASSET BALANCES:")
		assetBalances, err := client.AssetBalances(ctx)
		if err != nil {
			return err
		}

		for _, balance := range assetBalances {
			log.Infof("%T%+v", balance, balance)
		}

		log.Infof("ASSET CURRENCIES:")
		currencies, err := client.AssetCurrencies(ctx)
		if err != nil {
			return err
		}

		for _, currency := range currencies {
			log.Infof("%T%+v", currency, currency)
		}

		log.Infof("MARKET TICKERS:")
		tickers, err := client.NewMarketTickersRequest(string(okexapi.InstrumentTypeSpot)).Do(ctx)
		if err != nil {
			return err
		}

		for _, ticker := range tickers {
			log.Infof("%T%+v", ticker, ticker)
		}

		ticker, err := client.NewMarketTickerRequest("ETH-USDT").Do(ctx)
		if err != nil {
			return err
		}
		log.Infof("TICKER:")
		log.Infof("%T%+v", ticker, ticker)

		log.Infof("PLACING ORDER:")
		placeResponse, err := client.NewPlaceOrderRequest().
			InstrumentID("LTC-USDT").
			OrderType(okexapi.OrderTypeLimit).
			Side(okexapi.SideTypeBuy).
			Price("50.0").
			Size("0.5").
			Do(ctx)
		if err != nil {
			return err
		}

		log.Infof("place order response: %+v", placeResponse)
		time.Sleep(time.Second)

		log.Infof("getting order detail...")
		orderDetail, err := client.NewGetOrderDetailsRequest().
			InstrumentID("LTC-USDT").
			OrderID(placeResponse[0].OrderID).
			Do(ctx)
		if err != nil {
			return err
		}

		log.Infof("order detail: %+v", orderDetail)

		cancelResponse, err := client.NewCancelOrderRequest().
			InstrumentID("LTC-USDT").
			OrderID(placeResponse[0].OrderID).
			Do(ctx)
		if err != nil {
			return err
		}
		log.Infof("cancel order response: %+v", cancelResponse)

		time.Sleep(time.Second)

		log.Infof("BATCH PLACE ORDER:")
		batchPlaceReq := client.NewBatchPlaceOrderRequest()
		batchPlaceReq.Add(client.NewPlaceOrderRequest().
			InstrumentID("LTC-USDT").
			OrderType(okexapi.OrderTypeLimit).
			Side(okexapi.SideTypeBuy).
			Price("50.0").
			Size("0.5"))

		batchPlaceReq.Add(client.NewPlaceOrderRequest().
			InstrumentID("LTC-USDT").
			OrderType(okexapi.OrderTypeLimit).
			Side(okexapi.SideTypeBuy).
			Price("30.0").
			Size("0.5"))

		batchPlaceResponse, err := batchPlaceReq.Do(ctx)
		if err != nil {
			return err
		}

		log.Infof("batch place order response: %+v", batchPlaceResponse)
		time.Sleep(time.Second)

		log.Infof("getting pending orders...")
		pendingOrders, err := client.NewGetOpenOrdersRequest().Do(ctx)
		if err != nil {
			return err
		}
		for _, pendingOrder := range pendingOrders {
			log.Infof("pending order: %+v", pendingOrder)
		}

		cancelReq := client.NewBatchCancelOrderRequest()
		for _, resp := range batchPlaceResponse {
			cancelReq.Add(client.NewCancelOrderRequest().
				InstrumentID("LTC-USDT").
				OrderID(resp.OrderID))
		}

		batchCancelResponse, err := cancelReq.Do(ctx)
		if err != nil {
			return err
		}
		log.Infof("batch cancel order response: %+v", batchCancelResponse)

		// cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

func main() {
	if _, err := os.Stat(".env.local"); err == nil {
		if err := godotenv.Load(".env.local"); err != nil {
			log.Fatal(err)
		}
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.WithError(err).Error("bind pflags error")
	}

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		log.WithError(err).Error("cmd error")
	}
}

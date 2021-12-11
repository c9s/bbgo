package main

import (
	"context"

	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// go run ./examples/kucoin orders
var ordersCmd = &cobra.Command{
	Use: "orders",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		req := client.TradeService.NewListOrdersRequest()

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		if len(symbol) == 0 {
			return errors.New("--symbol option is required")
		}

		req.Symbol(symbol)


		status, err := cmd.Flags().GetString("status")
		if err != nil {
			return err
		}

		if len(status) > 0 {
			req.Status(status)
		}

		page, err := req.Do(context.Background())
		if err != nil {
			return err
		}

		logrus.Infof("page: %+v", page)
		return nil
	},
}

func init() {
	ordersCmd.Flags().String("symbol", "", "symbol, BTC-USDT, LTC-USDT...etc")
	ordersCmd.Flags().String("status", "", "status, active or done")
	rootCmd.AddCommand(ordersCmd)

	placeOrderCmd.Flags().String("symbol", "", "symbol")
	placeOrderCmd.Flags().String("price", "", "price")
	placeOrderCmd.Flags().String("size", "", "size")
	placeOrderCmd.Flags().String("order-type", string(kucoinapi.OrderTypeLimit), "order type")
	placeOrderCmd.Flags().String("side", "", "buy or sell")
	ordersCmd.AddCommand(placeOrderCmd)
}

// usage:
// go run ./examples/kucoin orders place --symbol LTC-USDT --price 50 --size 1 --order-type limit --side buy
var placeOrderCmd = &cobra.Command{
	Use: "place",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		req := client.TradeService.NewPlaceOrderRequest()

		orderType, err := cmd.Flags().GetString("order-type")
		if err != nil {
			return err
		}

		req.OrderType(kucoinapi.OrderType(orderType))


		side, err := cmd.Flags().GetString("side")
		if err != nil {
			return err
		}
		req.Side(kucoinapi.SideType(side))


		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		if len(symbol) == 0 {
			return errors.New("--symbol is required")
		}

		req.Symbol(symbol)

		switch kucoinapi.OrderType(orderType) {
		case kucoinapi.OrderTypeLimit:
			price, err := cmd.Flags().GetString("price")
			if err != nil {
				return err
			}
			req.Price(price)

		case kucoinapi.OrderTypeMarket:

		}


		size, err := cmd.Flags().GetString("size")
		if err != nil {
			return err
		}
		req.Size(size)

		response, err := req.Do(context.Background())
		if err != nil {
			return err
		}

		logrus.Infof("place order response: %+v", response)
		return nil
	},
}


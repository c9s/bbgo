package main

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	fillsCmd.Flags().String("symbol", "", "symbol, BTC-USDT, LTC-USDT...etc")
	rootCmd.AddCommand(fillsCmd)
}

// go run ./examples/kucoin fills
var fillsCmd = &cobra.Command{
	Use: "fills",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		if len(symbol) == 0 {
			return errors.New("--symbol option is required")
		}

		req := client.TradeService.NewGetFillsRequest()
		req.Symbol(symbol)

		page, err := req.Do(context.Background())
		if err != nil {
			return err
		}

		logrus.Infof("page: %+v", page)
		return nil
	},
}

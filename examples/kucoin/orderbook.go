package main

import (
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(orderbookCmd)
}

var orderbookCmd = &cobra.Command{
	Use: "orderbook",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	Args: cobra.MinimumNArgs(1),

	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}

		var depth = 0
		if len(args) > 1 {
			v, err := strconv.Atoi(args[1])
			if err != nil {
				return err
			}
			depth = v
		}

		orderBook, err := client.MarketDataService.GetOrderBook(args[0], depth)
		if err != nil {
			return err
		}

		logrus.Infof("orderBook: %+v", orderBook)
		return nil
	},
}

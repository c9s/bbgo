package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var tickersCmd = &cobra.Command{
	Use: "tickers",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			allTickers, err := client.MarketDataService.ListTickers()
			if err != nil {
				return err
			}

			logrus.Infof("allTickers: %+v", allTickers)
			return nil
		}


		ticker, err := client.MarketDataService.GetTicker(args[0])
		if err != nil {
			return err
		}

		logrus.Infof("ticker: %+v", ticker)
		return nil
	},
}


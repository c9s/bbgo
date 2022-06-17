package main

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(tickersCmd)
}

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

		req := client.MarketDataService.NewGetTickerRequest(args[0])
		ticker, err := req.Do(context.Background())
		if err != nil {
			return err
		}

		logrus.Infof("ticker: %+v", ticker)

		tickerStats, err := client.MarketDataService.GetTicker24HStat(args[0])
		if err != nil {
			return err
		}

		logrus.Infof("ticker 24h stats: %+v", tickerStats)
		return nil
	},
}

package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(symbolsCmd)
}

var symbolsCmd = &cobra.Command{
	Use: "symbols",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		symbols, err := client.MarketDataService.ListSymbols(args...)
		if err != nil {
			return err
		}

		logrus.Infof("symbols: %+v", symbols)
		return nil
	},
}

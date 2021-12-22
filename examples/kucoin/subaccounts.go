package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(subAccountsCmd)
}

var subAccountsCmd = &cobra.Command{
	Use:   "subaccounts",
	Short: "subaccounts",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		subAccounts, err := client.AccountService.QuerySubAccounts()
		if err != nil {
			return err
		}

		logrus.Infof("subAccounts: %+v", subAccounts)
		return nil
	},
}

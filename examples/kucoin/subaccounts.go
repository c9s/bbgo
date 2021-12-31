package main

import (
	"context"

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
		req := client.AccountService.NewListSubAccountsRequest()
		subAccounts, err := req.Do(context.Background())
		if err != nil {
			return err
		}

		logrus.Infof("subAccounts: %+v", subAccounts)
		return nil
	},
}

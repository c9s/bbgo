package main

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(accountsCmd)
}

var accountsCmd = &cobra.Command{
	Use: "accounts",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			account, err := client.AccountService.GetAccount(args[0])
			if err != nil {
				return err
			}

			logrus.Infof("account: %+v", account)
			return nil
		}

		accounts, err := client.AccountService.ListAccounts(context.Background())
		if err != nil {
			return err
		}

		logrus.Infof("accounts: %+v", accounts)
		return nil
	},
}


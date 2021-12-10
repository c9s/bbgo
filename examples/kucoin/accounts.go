package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var accountsCmd = &cobra.Command{
	Use: "accounts",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		accounts, err := client.AccountService.QueryAccounts()
		if err != nil {
			return err
		}

		logrus.Infof("accounts: %+v", accounts)
		return nil
	},
}

package cmd

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
	balancesCmd.Flags().String("session", "", "the exchange session name for querying balances")
	RootCmd.AddCommand(balancesCmd)
}

// go run ./cmd/bbgo balances --session=binance
var balancesCmd = &cobra.Command{
	Use:          "balances [--session SESSION]",
	Short:        "Show user account balances",
	SilenceUsage: true,
	PreRunE: cobraInitRequired([]string{
		"session",
	}),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()

		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		if len(sessionName) > 0 {
			session, ok := environ.Session(sessionName)
			if !ok {
				return fmt.Errorf("session %s not found", sessionName)
			}

			b, err := session.Exchange.QueryAccountBalances(ctx)
			if err != nil {
				return err
			}

			b.Print()
		} else {
			for _, session := range environ.Sessions() {

				b, err := session.Exchange.QueryAccountBalances(ctx)
				if err != nil {
					return err
				}

				log.Infof("SESSION %s", session.Name)
				b.Print()
			}
		}

		return nil
	},
}

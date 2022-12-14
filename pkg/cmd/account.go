package cmd

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/types"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
	accountCmd.Flags().String("session", "", "the exchange session name for querying information")
	accountCmd.Flags().Bool("total", false, "report total asset")
	RootCmd.AddCommand(accountCmd)
}

// go run ./cmd/bbgo account --session=binance --config=config/bbgo.yaml
var accountCmd = &cobra.Command{
	Use:          "account [--session SESSION]",
	Short:        "show user account details (ex: balance)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		showTotal, err := cmd.Flags().GetBool("total")
		if err != nil {
			return err
		}

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()
		if err := environ.ConfigureDatabase(ctx); err != nil {
			return err
		}

		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		if len(sessionName) > 0 {
			session, ok := environ.Session(sessionName)
			if !ok {
				return fmt.Errorf("session %s not found", sessionName)
			}

			a, err := session.Exchange.QueryAccount(ctx)
			if err != nil {
				return errors.Wrapf(err, "account query failed")
			}

			a.Print()
		} else {
			var total = types.BalanceMap{}
			for _, session := range environ.Sessions() {
				a, err := session.Exchange.QueryAccount(ctx)
				if err != nil {
					return errors.Wrapf(err, "account query failed")
				}

				log.Infof("--------------------------------------------")
				log.Infof("SESSION %s", session.Name)
				log.Infof("--------------------------------------------")
				a.Print()

				for c, b := range a.Balances() {
					tb, ok := total[c]
					if !ok {
						total[c] = b
					} else {
						tb.Available = tb.Available.Add(b.Available)
						tb.Locked = tb.Locked.Add(b.Locked)
						total[c] = tb
					}
				}

				if showTotal {
					log.Infof("===============================================")
					log.Infof("TOTAL ASSETS")
					log.Infof("===============================================")
					total.Print()
				}
			}

		}

		return nil
	},
}

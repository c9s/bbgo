package cmd

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
	accountCmd.Flags().String("session", "", "the exchange session name for querying information")
	RootCmd.AddCommand(accountCmd)
}

func printSessionAccount(ctx context.Context, session *bbgo.ExchangeSession) error {
	a, err := session.Exchange.QueryAccount(ctx)
	if err != nil {
		return errors.Wrapf(err, "account query failed")
	}

	log.Infof("--------------------------------------------")
	log.Infof("SESSION %s", session.Name)
	log.Infof("--------------------------------------------")
	a.Print(log.Infof)
	return nil
}

// go run ./cmd/bbgo account --session=binance --config=config/bbgo.yaml
var accountCmd = &cobra.Command{
	Use:          "account [--session SESSION]",
	Short:        "show user account details (ex: balance)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()
		if err := environ.ConfigureDatabase(ctx, userConfig); err != nil {
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

			if err := printSessionAccount(ctx, session); err != nil {
				return err
			}
		} else {
			for _, session := range environ.Sessions() {
				if err := printSessionAccount(ctx, session); err != nil {
					return err
				}
			}
		}

		return nil
	},
}

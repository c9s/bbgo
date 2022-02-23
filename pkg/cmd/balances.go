package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
	balancesCmd.Flags().String("session", "", "the exchange session name for querying balances")
	RootCmd.AddCommand(balancesCmd)
}

// go run ./cmd/bbgo balances --session=ftx
var balancesCmd = &cobra.Command{
	Use:          "balances",
	Short:        "Show user account balances",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		if len(configFile) == 0 {
			return errors.New("--config option is required")
		}

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		// if config file exists, use the config loaded from the config file.
		// otherwise, use a empty config object
		var userConfig *bbgo.Config
		if _, err := os.Stat(configFile); err == nil {
			// load successfully
			userConfig, err = bbgo.Load(configFile, false)
			if err != nil {
				return err
			}
		} else if os.IsNotExist(err) {
			// config file doesn't exist
			userConfig = &bbgo.Config{}
		} else {
			// other error
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

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
	accountCmd.Flags().String("session", "", "the exchange session name for querying information")
	RootCmd.AddCommand(accountCmd)
}

// go run ./cmd/bbgo account --session=ftx --config=config/bbgo.yaml
var accountCmd = &cobra.Command{
	Use:          "account",
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
			for _, session := range environ.Sessions() {
				a, err := session.Exchange.QueryAccount(ctx)
				if err != nil {
					return errors.Wrapf(err, "account query failed")
				}

				log.Infof("SESSION %s", session.Name)
				a.Print()
			}
		}

		return nil
	},
}

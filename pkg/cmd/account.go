package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
	accountCmd.Flags().String("session", "", "the exchange session name for querying balances")
	RootCmd.AddCommand(accountCmd)
}

// go run ./cmd/bbgo account --session=ftx
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

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		session, ok := environ.Session(sessionName)
		if !ok {
			return fmt.Errorf("session %s not found", sessionName)
		}

		account, err := session.Exchange.QueryAccount(ctx)
		if err != nil {
			return err
		}

		out, err := json.MarshalIndent(account, "", "  ")
		if err != nil {
			return err
		}

		log.Info(string(out))
		return nil
	},
}

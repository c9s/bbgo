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
	marketCmd.Flags().String("session", "", "the exchange session name for querying information")
	RootCmd.AddCommand(marketCmd)
}

// go run ./cmd/bbgo market --session=binance --config=config/bbgo.yaml
var marketCmd = &cobra.Command{
	Use:          "market",
	Short:        "List the symbols that the are available to be traded in the exchange",
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

		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			return err
		}

		userConfig, err := bbgo.Load(configFile, false)
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

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		session, ok := environ.Session(sessionName)
		if !ok {
			return fmt.Errorf("session %s not found", sessionName)
		}

		markets, err := session.Exchange.QueryMarkets(ctx)
		if err != nil {
			return err
		}

		for _, m := range markets {
			log.Infof("market: %+v", m)
		}
		return nil
	},
}

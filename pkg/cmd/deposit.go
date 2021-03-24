package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	depositsCmd.Flags().String("session", "", "the exchange session name for querying balances")
	depositsCmd.Flags().String("asset", "", "the trading pair, like btcusdt")
	RootCmd.AddCommand(depositsCmd)
}

// go run ./cmd/bbgo deposits --session=ftx --asset="BTC"
// This is a testing util and will query deposits in last 7 days.
var depositsCmd = &cobra.Command{
	Use:          "deposits",
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

		asset, err := cmd.Flags().GetString("asset")
		if err != nil {
			return fmt.Errorf("can't get the asset from flags: %w", err)
		}
		if asset == "" {
			return fmt.Errorf("asset is not found")
		}

		until := time.Now()
		since := until.Add(-7 * 24 * time.Hour)
		exchange, ok := session.Exchange.(types.ExchangeTransferService)
		if !ok {
			return fmt.Errorf("exchange session %s does not implement transfer service", sessionName)
		}
		histories, err := exchange.QueryDepositHistory(ctx, asset, since, until)
		if err != nil {
			return err
		}

		log.Infof("%d histories", len(histories))
		for _, h := range histories {
			log.Infof("deposit history: %+v", h)
		}
		return nil
	},
}

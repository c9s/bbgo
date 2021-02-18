package cmd

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
	SyncCmd.Flags().String("session", "", "the exchange session name for sync")
	SyncCmd.Flags().String("symbol", "", "symbol of market for syncing")
	SyncCmd.Flags().String("since", "", "sync from time")
	RootCmd.AddCommand(SyncCmd)
}

var SyncCmd = &cobra.Command{
	Use:          "sync",
	Short:        "sync trades, orders",
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

		since, err := cmd.Flags().GetString("since")
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()
		if err := configureDB(ctx, environ); err != nil {
			return err
		}

		if err := environ.AddExchangesFromConfig(userConfig); err != nil {
			return err
		}

		var (
			// default start time
			startTime = time.Now().AddDate(0, -3, 0)
		)

		if len(since) > 0 {
			loc, err := time.LoadLocation("Asia/Taipei")
			if err != nil {
				return err
			}

			startTime, err = time.ParseInLocation("2006-01-02", since, loc)
			if err != nil {
				return err
			}
		}

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		var defaultSymbols []string
		if len(symbol) > 0 {
			defaultSymbols = []string{symbol}
		}

		var selectedSessions []string

		if len(sessionName) > 0 {
			selectedSessions = []string{sessionName}
		}

		sessions := environ.SelectSessions(selectedSessions...)
		for _, session := range sessions {
			if err := session.Init(ctx, environ) ; err != nil {
				return err
			}

			if err := bbgo.SyncSession(ctx, environ, session, startTime, defaultSymbols...) ; err != nil {
				return err
			}

			log.Infof("exchange session %s synchronization done", session.Name)
		}

		return nil
	},
}


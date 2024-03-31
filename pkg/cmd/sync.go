package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
	SyncCmd.Flags().StringArray("session", []string{}, "the exchange session name for sync")
	SyncCmd.Flags().String("symbol", "", "symbol of market for syncing")
	SyncCmd.Flags().String("since", "", "sync from time")
	RootCmd.AddCommand(SyncCmd)
}

var SyncCmd = &cobra.Command{
	Use:          "sync [--session=[exchange_name]] [--symbol=[pair_name]] [[--since=yyyy/mm/dd]]",
	Short:        "sync trades and orders history",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		since, err := cmd.Flags().GetString("since")
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

		sessionNames, err := cmd.Flags().GetStringArray("session")
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		var (
			// default sync start time
			defaultSyncStartTime = time.Now().AddDate(-1, 0, 0)
		)

		var syncStartTime = defaultSyncStartTime

		if userConfig.Sync != nil && userConfig.Sync.Since != nil {
			syncStartTime = userConfig.Sync.Since.Time()
		}

		if len(since) > 0 {
			syncStartTime, err = time.ParseInLocation("2006-01-02", since, time.Local)
			if err != nil {
				return err
			}
		}

		environ.SetSyncStartTime(syncStartTime)

		if len(symbol) > 0 {
			if userConfig.Sync != nil && len(userConfig.Sync.Symbols) > 0 {
				userConfig.Sync.Symbols = []bbgo.SyncSymbol{
					{Symbol: symbol},
				}
			}
		}

		if len(sessionNames) > 0 {
			if userConfig.Sync != nil && len(userConfig.Sync.Sessions) > 0 {
				userConfig.Sync.Sessions = sessionNames
			}
		}

		return environ.Sync(ctx, userConfig)
	},
}

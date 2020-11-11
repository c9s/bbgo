package cmd

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	SyncCmd.Flags().String("exchange", "", "target exchange")
	SyncCmd.Flags().String("symbol", "BTCUSDT", "trading symbol")
	SyncCmd.Flags().String("since", "", "sync from time")
	RootCmd.AddCommand(SyncCmd)
}

var SyncCmd = &cobra.Command{
	Use:          "sync",
	Short:        "sync trades, orders",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		exchangeNameStr, err := cmd.Flags().GetString("exchange")
		if err != nil {
			return err
		}

		exchangeName, err := types.ValidExchangeName(exchangeNameStr)
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		exchange, err := cmdutil.NewExchange(exchangeName)
		if err != nil {
			return err
		}

		db, err := cmdutil.ConnectMySQL()
		if err != nil {
			return err
		}

		since, err := cmd.Flags().GetString("since")
		if err != nil {
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

		tradeService := &service.TradeService{DB: db}
		orderService := &service.OrderService{DB: db}
		syncService := &service.SyncService{
			TradeService: tradeService,
			OrderService: orderService,
		}

		log.Info("syncing trades from exchange...")
		if err := syncService.SyncTrades(ctx, exchange, symbol, startTime); err != nil {
			return err
		}

		log.Info("syncing orders from exchange...")
		if err := syncService.SyncOrders(ctx, exchange, symbol, startTime); err != nil {
			return err
		}

		log.Info("synchronization done")

		return nil
	},
}

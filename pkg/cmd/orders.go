package cmd

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/types"
)

// go run ./cmd/bbgo orders [open|closed] --session=ftx --symbol=BTC/USDT
var ordersCmd = &cobra.Command{
	Use:  "orders [status]",
	Args: cobra.OnlyValidArgs,
	// default is open which means we query open orders if you haven't provided args.
	ValidArgs:    []string{"", "open", "closed"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		session, err := cmd.Flags().GetString("session")
		if err != nil {
			return fmt.Errorf("can't get session from flags: %w", err)
		}
		ex, err := newExchange(session)
		if err != nil {
			return err
		}
		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return fmt.Errorf("can't get the symbol from flags: %w", err)
		}
		if symbol == "" {
			return fmt.Errorf("symbol is not found")
		}

		status := "open"
		if len(args) != 0 {
			status = args[0]
		}

		var os []types.Order
		switch status {
		case "open":
			os, err = ex.QueryOpenOrders(ctx, symbol)
			if err != nil {
				return err
			}
		case "closed":
		default:
			return fmt.Errorf("invalid status %s", status)
		}
		log.Infof("%s orders: %+v", status, os)

		return nil
	},
}

func init() {
	ordersCmd.Flags().String("session", "", "the exchange session name for sync")
	ordersCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")

	RootCmd.AddCommand(ordersCmd)
}

package cmd

import (
	"context"
	"fmt"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
)

// go run ./cmd/bbgo orderbook --session=ftx --symbol=BTC/USDT
var orderbookCmd = &cobra.Command{
	Use: "orderbook",
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

		s := ex.NewStream()
		s.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})
		s.OnBookSnapshot(func(book types.OrderBook) {
			log.Infof("orderbook snapshot: %s", book.String())
		})
		s.OnBookUpdate(func(book types.OrderBook) {
			log.Infof("orderbook update: %s", book.String())
		})

		if err := s.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect to %s", session)
		}

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

func init() {
	orderbookCmd.Flags().String("session", "", "the exchange session name for sync")
	orderbookCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")

	RootCmd.AddCommand(orderbookCmd)
}

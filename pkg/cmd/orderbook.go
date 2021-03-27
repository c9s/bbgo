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

// go run ./cmd/bbgo orderbook --exchange=ftx --symbol=BTC/USDT
var orderbookCmd = &cobra.Command{
	Use:   "orderbook",
	Short: "connect to the order book market data streaming service of an exchange",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		exName, err := cmd.Flags().GetString("exchange")
		if err != nil {
			return fmt.Errorf("can not get exchange from flags: %w", err)
		}

		exchangeName, err := types.ValidExchangeName(exName)
		if err != nil {
			return err
		}

		ex, err := cmdutil.NewExchange(exchangeName)
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return fmt.Errorf("can not get the symbol from flags: %w", err)
		}

		if symbol == "" {
			return fmt.Errorf("--symbol option is required")
		}

		s := ex.NewStream()
		s.SetPublicOnly()
		s.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})
		s.OnBookSnapshot(func(book types.OrderBook) {
			log.Infof("orderbook snapshot: %s", book.String())
		})
		s.OnBookUpdate(func(book types.OrderBook) {
			log.Infof("orderbook update: %s", book.String())
		})

		log.Infof("connecting...")
		if err := s.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect to %s", exchangeName)
		}

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

func init() {
	// since the public data does not require trading authentication, we use --exchange option here.
	orderbookCmd.Flags().String("exchange", "", "the exchange name for sync")
	orderbookCmd.Flags().String("symbol", "", "the trading pair. e.g, BTCUSDT, LTCUSDT...")
	RootCmd.AddCommand(orderbookCmd)
}

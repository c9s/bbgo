package cmd

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
)

// go run ./cmd/bbgo orderbook --exchange=ftx --symbol=BTCUSDT
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
		s.OnBookSnapshot(func(book types.SliceOrderBook) {
			log.Infof("orderbook snapshot: %s", book.String())
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
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

// go run ./cmd/bbgo orderupdate --session=ftx
var orderUpdateCmd = &cobra.Command{
	Use: "orderupdate",
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

		s := session.Exchange.NewStream()
		s.OnOrderUpdate(func(order types.Order) {
			log.Infof("order update: %+v", order)
		})

		log.Infof("connecting...")
		if err := s.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect to %s", sessionName)
		}
		log.Infof("connected")

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

func init() {
	// since the public data does not require trading authentication, we use --exchange option here.
	orderbookCmd.Flags().String("exchange", "", "the exchange name for sync")
	orderbookCmd.Flags().String("symbol", "", "the trading pair. e.g, BTCUSDT, LTCUSDT...")

	orderUpdateCmd.Flags().String("session", "", "session name")
	RootCmd.AddCommand(orderbookCmd)
	RootCmd.AddCommand(orderUpdateCmd)
}

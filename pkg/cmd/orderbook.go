package cmd

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
)

// go run ./cmd/bbgo orderbook --session=ftx --symbol=BTCUSDT
var orderbookCmd = &cobra.Command{
	Use:   "orderbook",
	Short: "connect to the order book market data streaming service of an exchange",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		sessionName, err := cmd.Flags().GetString("session")
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

		if userConfig == nil {
			return errors.New("--config option or config file is missing")
		}

		environ := bbgo.NewEnvironment()
		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		session, ok := environ.Session(sessionName)
		if !ok {
			return fmt.Errorf("session %s not found", sessionName)
		}

		orderBook := types.NewMutexOrderBook(symbol)

		s := session.Exchange.NewStream()
		s.SetPublicOnly()
		s.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})
		s.OnBookSnapshot(func(book types.SliceOrderBook) {
			log.Infof("orderbook snapshot: %s", book.String())
			orderBook.Load(book)

			if ok, err := orderBook.IsValid() ; !ok {
				log.WithError(err).Panicf("invalid error book snapshot")
			}

			if bid, ask, ok := orderBook.BestBidAndAsk() ; ok {
				log.Infof("ASK | %f x %f / %f x %f | BID",
					ask.Volume.Float64(), ask.Price.Float64(),
					bid.Price.Float64(), bid.Volume.Float64())
			}
		})
		s.OnBookUpdate(func(book types.SliceOrderBook) {
			log.Infof("orderbook update: %s", book.String())

			orderBook.Update(book)

			if ok, err := orderBook.IsValid() ; !ok {
				log.WithError(err).Panicf("invalid error book update")
			}

			if bid, ask, ok := orderBook.BestBidAndAsk() ; ok {
				log.Infof("ASK | %f x %f / %f x %f | BID",
					ask.Volume.Float64(), ask.Price.Float64(),
					bid.Price.Float64(), bid.Volume.Float64())
			}
		})

		log.Infof("connecting...")
		if err := s.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect to %s", sessionName)
		}

		log.Infof("connected")
		defer func() {
			log.Infof("closing connection...")
			if err := s.Close(); err != nil {
				log.WithError(err).Errorf("connection close error")
			}
			time.Sleep(1 * time.Second)
		}()

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

// go run ./cmd/bbgo orderupdate --session=ftx
var orderUpdateCmd = &cobra.Command{
	Use: "orderupdate",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if userConfig == nil {
			return errors.New("--config option or config file is missing")
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
	orderbookCmd.Flags().String("session", "", "session name")
	orderbookCmd.Flags().String("symbol", "", "the trading pair. e.g, BTCUSDT, LTCUSDT...")

	orderUpdateCmd.Flags().String("session", "", "session name")
	RootCmd.AddCommand(orderbookCmd)
	RootCmd.AddCommand(orderUpdateCmd)
}

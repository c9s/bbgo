package cmd

import (
	"context"
	"fmt"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
)

// go run ./cmd/bbgo kline --exchange=ftx --symbol=BTCUSDT
var klineCmd = &cobra.Command{
	Use:   "kline",
	Short: "connect to the kline market data streaming service of an exchange",
	PreRunE: cobraInitRequired([]string{
		"session",
		"symbol",
		"interval",
	}),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

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

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return fmt.Errorf("can not get the symbol from flags: %w", err)
		}

		if symbol == "" {
			return fmt.Errorf("--symbol option is required")
		}

		interval, err := cmd.Flags().GetString("interval")
		if err != nil {
			return err
		}

		s := session.Exchange.NewStream()
		s.SetPublicOnly()
		s.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: interval})

		s.OnKLineClosed(func(kline types.KLine) {
			log.Infof("kline closed: %s", kline.String())
		})

		s.OnKLine(func(kline types.KLine) {
			log.Infof("kline: %s", kline.String())
		})

		log.Infof("connecting...")
		if err := s.Connect(ctx); err != nil {
			return err
		}

		log.Infof("connected")
		defer func() {
			log.Infof("closing connection...")
			if err := s.Close(); err != nil {
				log.WithError(err).Errorf("connection close error")
			}
		}()

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

func init() {
	// since the public data does not require trading authentication, we use --exchange option here.
	klineCmd.Flags().String("session", "", "session name")
	klineCmd.Flags().String("symbol", "", "the trading pair. e.g, BTCUSDT, LTCUSDT...")
	klineCmd.Flags().String("interval", "1m", "interval of the kline (candle), .e.g, 1m, 3m, 15m")
	RootCmd.AddCommand(klineCmd)
}

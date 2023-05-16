package cmd

import (
	"context"
	"fmt"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
)

// go run ./cmd/bbgo kline --exchange=binance --symbol=BTCUSDT
var exchangeTestCmd = &cobra.Command{
	Use:   "exchange-test",
	Short: "test the exchange",
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

		now := time.Now()
		kLines, err := session.Exchange.QueryKLines(ctx, symbol, types.Interval(interval), types.KLineQueryOptions{
			Limit:   50,
			EndTime: &now,
		})
		if err != nil {
			return err
		}
		log.Infof("kLines from RESTful API")
		for _, k := range kLines {
			log.Info(k.String())
		}

		s := session.Exchange.NewStream()
		s.SetPublicOnly()
		s.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: types.Interval(interval)})

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
	exchangeTestCmd.Flags().String("session", "", "session name")
	exchangeTestCmd.Flags().String("symbol", "", "the trading pair. e.g, BTCUSDT, LTCUSDT...")
	exchangeTestCmd.Flags().String("interval", "1m", "interval of the kline (candle), .e.g, 1m, 3m, 15m")
	RootCmd.AddCommand(exchangeTestCmd)
}

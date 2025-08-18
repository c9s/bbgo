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

		symbols, err := cmd.Flags().GetStringArray("symbol")
		if err != nil {
			return fmt.Errorf("can not get the symbol from flags: %w", err)
		}

		if len(symbols) == 0 {
			return fmt.Errorf("--symbol option is required")
		}

		interval, err := cmd.Flags().GetString("interval")
		if err != nil {
			return err
		}

		now := time.Now()
		s := session.Exchange.NewStream()
		s.SetPublicOnly()
		for _, symbol := range symbols {
			kLines, err := session.Exchange.QueryKLines(ctx, symbol, types.Interval(interval), types.KLineQueryOptions{
				Limit:   5,
				EndTime: &now,
			})
			if err != nil {
				return err
			}
			log.Infof("kLines from RESTful API")
			for _, k := range kLines {
				log.Info(k.String())
			}
			s.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: types.Interval(interval)})
		}
		s.OnKLineClosed(func(kline types.KLine) {
			log.Infof("kline closed: %s (%s~%s)",
				kline.String(),
				kline.StartTime.Time().Format(time.RFC3339),
				kline.EndTime.Time().Format(time.RFC3339),
			)
		})

		s.OnKLine(func(kline types.KLine) {
			log.Infof("kline: %s (%s~%s)", kline.String(),
				kline.StartTime.Time().Format(time.RFC3339),
				kline.EndTime.Time().Format(time.RFC3339),
			)
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
	klineCmd.Flags().StringArray("symbol", []string{""}, "the trading pair. e.g, BTCUSDT, LTCUSDT...")
	klineCmd.Flags().String("interval", "1m", "interval of the kline (candle), .e.g, 1m, 3m, 15m")
	RootCmd.AddCommand(klineCmd)
}

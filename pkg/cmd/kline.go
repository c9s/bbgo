package cmd

import (
	"context"
	"fmt"
	"syscall"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// go run ./cmd/bbgo kline --exchange=ftx --symbol=BTCUSDT
var klineCmd = &cobra.Command{
	Use:   "kline",
	Short: "connect to the kline market data streaming service of an exchange",
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

		interval, err := cmd.Flags().GetString("interval")
		if err != nil {
			return err
		}

		s := ex.NewStream()
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
			return fmt.Errorf("failed to connect to %s", exchangeName)
		}

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

func init() {
	// since the public data does not require trading authentication, we use --exchange option here.
	klineCmd.Flags().String("exchange", "", "the exchange name")
	klineCmd.Flags().String("symbol", "", "the trading pair. e.g, BTCUSDT, LTCUSDT...")
	klineCmd.Flags().String("interval", "1m", "interval of the kline (candle), .e.g, 1m, 3m, 15m")
	RootCmd.AddCommand(klineCmd)
}

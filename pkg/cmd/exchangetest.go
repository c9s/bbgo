//go:build exchangetest
// +build exchangetest

package cmd

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/types"
)

// go run ./cmd/bbgo kline --exchange=binance --symbol=BTCUSDT
var exchangeTestCmd = &cobra.Command{
	Use:   "exchange-test",
	Short: "test the exchange",
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

		exMinimal, err := exchange.NewWithEnvVarPrefix(exchangeName, "")
		if err != nil {
			return err
		}

		log.Infof("types.ExchangeMinimal: ✅")

		if service, ok := exMinimal.(types.ExchangeAccountService); ok {
			log.Infof("types.ExchangeAccountService: ✅ (%T)", service)
		}

		if service, ok := exMinimal.(types.ExchangeMarketDataService); ok {
			log.Infof("types.ExchangeMarketDataService: ✅ (%T)", service)
		}

		if ex, ok := exMinimal.(types.Exchange); ok {
			log.Infof("types.Exchange: ✅ (%T)", ex)
		}

		_ = ctx
		// cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		return nil
	},
}

func init() {
	exchangeTestCmd.Flags().String("exchange", "", "session name")
	exchangeTestCmd.MarkFlagRequired("exchange")

	RootCmd.AddCommand(exchangeTestCmd)
}

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

// go run ./cmd/bbgo trades --session=binance --symbol="BTC/USD"
var tradesCmd = &cobra.Command{
	Use:          "trades --session=[exchange_name] --symbol=[pair_name]",
	Short:        "Query trading history",
	SilenceUsage: true,
	PreRunE: cobraInitRequired([]string{
		"session",
		"symbol",
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
			return fmt.Errorf("can't get the symbol from flags: %w", err)
		}
		if symbol == "" {
			return fmt.Errorf("symbol is not found")
		}

		limit, err := cmd.Flags().GetInt64("limit")
		if err != nil {
			return err
		}

		now := time.Now()
		since := now.Add(-24 * time.Hour)

		tradeHistoryService, ok := session.Exchange.(types.ExchangeTradeHistoryService)
		if !ok {
			// skip exchanges that does not support trading history services
			log.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing closed orders (tradesCmd)", session.Exchange.Name())
			return nil
		}

		trades, err := tradeHistoryService.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
			StartTime:   &since,
			Limit:       limit,
			LastTradeID: 0,
		})
		if err != nil {
			return err
		}

		log.Infof("%d trades", len(trades))
		for _, trade := range trades {
			log.Infof("TRADE %s %s %4s %s @ %s orderID %d %s amount %v , fee %v %s ",
				trade.Exchange.String(),
				trade.Symbol,
				trade.Side,
				trade.Quantity.FormatString(4),
				trade.Price.FormatString(3),
				trade.OrderID,
				trade.Time.Time().Format(time.StampMilli),
				trade.QuoteQuantity,
				trade.Fee,
				trade.FeeCurrency)
		}
		return nil
	},
}

// go run ./cmd/bbgo tradeupdate --session=ftx
var tradeUpdateCmd = &cobra.Command{
	Use:   "tradeupdate --session=[exchange_name]",
	Short: "Listen to trade update events",
	PreRunE: cobraInitRequired([]string{
		"session",
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

		s := session.Exchange.NewStream()
		s.OnTradeUpdate(func(trade types.Trade) {
			log.Infof("trade update: %+v", trade)
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
	tradesCmd.Flags().String("session", "", "the exchange session name for querying balances")
	tradesCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")
	tradesCmd.Flags().Int64("limit", 100, "limit")

	tradeUpdateCmd.Flags().String("session", "", "the exchange session name for querying balances")

	RootCmd.AddCommand(tradesCmd)
	RootCmd.AddCommand(tradeUpdateCmd)
}

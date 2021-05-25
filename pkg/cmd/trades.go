package cmd

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/c9s/bbgo/pkg/util"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/types"
)

// go run ./cmd/bbgo tradesCmd --session=ftx --symbol="BTC/USD"
var tradesCmd = &cobra.Command{
	Use:          "trades",
	SilenceUsage: true,
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

		until := time.Now()
		since := until.Add(-3 * 24 * time.Hour)

		tradeHistoryService, ok := session.Exchange.(types.ExchangeTradeHistoryService)
		if !ok {
			// skip exchanges that does not support trading history services
			log.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing closed orders", session.Exchange.Name())
			return nil
		}

		trades, err := tradeHistoryService.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
			StartTime:   &since,
			EndTime:     &until,
			Limit:       limit,
			LastTradeID: 0,
		})
		if err != nil {
			return err
		}

		log.Infof("%d trades", len(trades))
		for _, trade := range trades {
			log.Infof("TRADE %s %s %4s %s @ %s orderID %d %s amount %f",
				trade.Exchange.String(),
				trade.Symbol,
				trade.Side,
				util.FormatFloat(trade.Quantity, 4),
				util.FormatFloat(trade.Price, 3),
				trade.OrderID,
				trade.Time.Time().Format(time.StampMilli),
				trade.QuoteQuantity)
		}
		return nil
	},
}

// go run ./cmd/bbgo tradeupdate --session=ftx
var tradeUpdateCmd = &cobra.Command{
	Use: "tradeupdate",
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

package cmd

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/ftx"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

// go run ./cmd/bbgo listorders [open|closed] --session=ftx --symbol=BTC/USDT
var listOrdersCmd = &cobra.Command{
	Use:  "listorders [status]",
	Args: cobra.OnlyValidArgs,
	// default is open which means we query open orders if you haven't provided args.
	ValidArgs:    []string{"", "open", "closed"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		if len(configFile) == 0 {
			return fmt.Errorf("--config option is required")
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

		status := "open"
		if len(args) != 0 {
			status = args[0]
		}

		var os []types.Order
		switch status {
		case "open":
			os, err = session.Exchange.QueryOpenOrders(ctx, symbol)
			if err != nil {
				return err
			}
		case "closed":
			os, err = session.Exchange.QueryClosedOrders(ctx, symbol, time.Now().Add(-3*24*time.Hour), time.Now(), 0)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid status %s", status)
		}

		for _, o := range os {
			log.Infof("%s orders: %+v", status, o)
		}

		return nil
	},
}

var executeOrderCmd = &cobra.Command{
	Use:          "execute-order",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if userConfig == nil {
			return errors.New("config file is required")
		}

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return fmt.Errorf("can not get the symbol from flags: %w", err)
		}

		if symbol == "" {
			return fmt.Errorf("symbol not found")
		}

		sideS, err := cmd.Flags().GetString("side")
		if err != nil {
			return fmt.Errorf("can't get side: %w", err)
		}

		side, err := types.StrToSideType(sideS)
		if err != nil {
			return err
		}

		targetQuantityS, err := cmd.Flags().GetString("target-quantity")
		if err != nil {
			return err
		}

		targetQuantity, err := fixedpoint.NewFromString(targetQuantityS)
		if err != nil {
			return err
		}

		sliceQuantityS, err := cmd.Flags().GetString("slice-quantity")
		if err != nil {
			return err
		}

		sliceQuantity, err := fixedpoint.NewFromString(sliceQuantityS)
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()
		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		if err := environ.Init(ctx); err != nil {
			return err
		}

		session, ok := environ.Session(sessionName)
		if !ok {
			return fmt.Errorf("session %s not found", sessionName)
		}

		executor := &bbgo.TwapOrderExecutor{
			Session: session,
		}

		execCtx, execCancel := context.WithCancel(ctx)

		execution, err := executor.Execute(execCtx, symbol, side, targetQuantity, sliceQuantity)
		if err != nil {
			execCancel()
			_ = execution
			return err
		}

		// report execution here...
		_ = execution

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)

		log.Infof("shutting down order executor...")
		execCancel()
		return nil
	},
}

// go run ./cmd/bbgo submit-order --session=ftx --symbol=BTC/USDT --side=buy --price=<price> --quantity=<quantity>
var submitOrderCmd = &cobra.Command{
	Use:          "submit-order",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if userConfig == nil {
			return errors.New("config file is required")
		}

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return fmt.Errorf("can't get the symbol from flags: %w", err)
		}
		if symbol == "" {
			return fmt.Errorf("symbol is not found")
		}

		side, err := cmd.Flags().GetString("side")
		if err != nil {
			return fmt.Errorf("can't get side: %w", err)
		}

		price, err := cmd.Flags().GetString("price")
		if err != nil {
			return fmt.Errorf("can't get price: %w", err)
		}

		quantity, err := cmd.Flags().GetString("quantity")
		if err != nil {
			return fmt.Errorf("can't get quantity: %w", err)
		}

		environ := bbgo.NewEnvironment()
		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		if err := environ.Init(ctx); err != nil {
			return err
		}

		session, ok := environ.Session(sessionName)
		if !ok {
			return fmt.Errorf("session %s not found", sessionName)
		}

		so := types.SubmitOrder{
			ClientOrderID:  uuid.New().String(),
			Symbol:         symbol,
			Side:           types.SideType(ftx.TrimUpperString(side)),
			Type:           types.OrderTypeLimit,
			Quantity:       util.MustParseFloat(quantity),
			QuantityString: quantity,
			Price:          util.MustParseFloat(price),
			PriceString:    price,
			Market:         types.Market{Symbol: symbol},
			TimeInForce:    "GTC",
		}
		co, err := session.Exchange.SubmitOrders(ctx, so)
		if err != nil {
			return err
		}

		log.Infof("submitted order: %+v\ncreated order: %+v", so, co[0])
		return nil
	},
}

func init() {
	listOrdersCmd.Flags().String("session", "", "the exchange session name for sync")
	listOrdersCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")

	submitOrderCmd.Flags().String("session", "", "the exchange session name for sync")
	submitOrderCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")
	submitOrderCmd.Flags().String("side", "", "the trading side: buy or sell")
	submitOrderCmd.Flags().String("price", "", "the trading price")
	submitOrderCmd.Flags().String("quantity", "", "the trading quantity")

	executeOrderCmd.Flags().String("session", "", "the exchange session name for sync")
	executeOrderCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")
	executeOrderCmd.Flags().String("side", "", "the trading side: buy or sell")
	executeOrderCmd.Flags().String("target-quantity", "", "target quantity")
	executeOrderCmd.Flags().String("slice-quantity", "", "slice quantity")

	RootCmd.AddCommand(listOrdersCmd)
	RootCmd.AddCommand(submitOrderCmd)
	RootCmd.AddCommand(executeOrderCmd)
}

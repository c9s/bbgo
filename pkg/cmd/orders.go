package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

var getOrderCmd = &cobra.Command{
	Use:          "get-order --session SESSION --order-id ORDER_ID",
	Short:        "Get order status",
	SilenceUsage: true,
	PreRunE: cobraInitRequired([]string{
		"order-id",
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

		orderID, err := cmd.Flags().GetString("order-id")
		if err != nil {
			return fmt.Errorf("can't get the order-id from flags: %w", err)
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return fmt.Errorf("can't get the symbol from flags: %w", err)
		}

		service, ok := session.Exchange.(types.ExchangeOrderQueryService)
		if !ok {
			return fmt.Errorf("query order status is not supported for exchange %T, interface types.ExchangeOrderQueryService is not implemented", session.Exchange)
		}

		order, err := service.QueryOrder(ctx, types.OrderQuery{
			OrderID: orderID,
			Symbol:  symbol,
		})
		if err != nil {
			return err
		}

		log.Infof("%+v", order)

		return nil
	},
}

// go run ./cmd/bbgo list-orders [open|closed] --session=ftx --symbol=BTCUSDT
var listOrdersCmd = &cobra.Command{
	Use:   "list-orders open|closed --session SESSION --symbol SYMBOL",
	Short: "list user's open orders in exchange of a specific trading pair",
	Args:  cobra.OnlyValidArgs,
	// default is open which means we query open orders if you haven't provided args.
	ValidArgs:    []string{"", "open", "closed"},
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
			tradeHistoryService, ok := session.Exchange.(types.ExchangeTradeHistoryService)
			if !ok {
				// skip exchanges that does not support trading history services
				log.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing closed orders (listOrdersCmd)", session.Exchange.Name())
				return nil
			}

			os, err = tradeHistoryService.QueryClosedOrders(ctx, symbol, time.Now().Add(-3*24*time.Hour), time.Now(), 0)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid status %s", status)
		}

		log.Infof("%s ORDERS FROM %s SESSION", strings.ToUpper(status), session.Name)
		for _, o := range os {
			log.Infof("%+v", o)
		}

		return nil
	},
}

var executeOrderCmd = &cobra.Command{
	Use:          "execute-order --session SESSION --symbol SYMBOL --side SIDE --target-quantity TOTAL_QUANTITY --slice-quantity SLICE_QUANTITY",
	Short:        "execute buy/sell on the balance/position you have on specific symbol",
	SilenceUsage: true,
	PreRunE: cobraInitRequired([]string{
		"symbol",
		"side",
		"target-quantity",
		"slice-quantity",
	}),
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
		if len(targetQuantityS) == 0 {
			return errors.New("--target-quantity can not be empty")
		}

		targetQuantity, err := fixedpoint.NewFromString(targetQuantityS)
		if err != nil {
			return err
		}

		sliceQuantityS, err := cmd.Flags().GetString("slice-quantity")
		if err != nil {
			return err
		}
		if len(sliceQuantityS) == 0 {
			return errors.New("--slice-quantity can not be empty")
		}

		sliceQuantity, err := fixedpoint.NewFromString(sliceQuantityS)
		if err != nil {
			return err
		}

		numOfPriceTicks, err := cmd.Flags().GetInt("price-ticks")
		if err != nil {
			return err
		}

		stopPriceS, err := cmd.Flags().GetString("stop-price")
		if err != nil {
			return err
		}

		stopPrice, err := fixedpoint.NewFromString(stopPriceS)
		if err != nil {
			return err
		}

		updateInterval, err := cmd.Flags().GetDuration("update-interval")
		if err != nil {
			return err
		}

		deadlineDuration, err := cmd.Flags().GetDuration("deadline")
		if err != nil {
			return err
		}

		var deadlineTime time.Time
		if deadlineDuration > 0 {
			deadlineTime = time.Now().Add(deadlineDuration)
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

		executionCtx, cancelExecution := context.WithCancel(ctx)
		defer cancelExecution()

		execution := &bbgo.TwapExecution{
			Session:        session,
			Symbol:         symbol,
			Side:           side,
			TargetQuantity: targetQuantity,
			SliceQuantity:  sliceQuantity,
			StopPrice:      stopPrice,
			NumOfTicks:     numOfPriceTicks,
			UpdateInterval: updateInterval,
			DeadlineTime:   deadlineTime,
		}

		if err := execution.Run(executionCtx); err != nil {
			return err
		}

		var sigC = make(chan os.Signal, 1)
		signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigC)

		select {
		case sig := <-sigC:
			log.Warnf("signal %v", sig)
			log.Infof("shutting down order executor...")
			shutdownCtx, cancelShutdown := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
			execution.Shutdown(shutdownCtx)
			cancelShutdown()

		case <-execution.Done():
			log.Infof("the order execution is completed")

		case <-ctx.Done():

		}

		return nil
	},
}

// go run ./cmd/bbgo submit-order --session=ftx --symbol=BTCUSDT --side=buy --price=18000 --quantity=0.001
var submitOrderCmd = &cobra.Command{
	Use:          "submit-order --session SESSION --symbol SYMBOL --side SIDE --quantity QUANTITY [--price PRICE]",
	Short:        "place order to the exchange",
	SilenceUsage: true,
	PreRunE: cobraInitRequired([]string{
		"session",
		"symbol",
		"side",
		"quantity",
	}),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

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
			return fmt.Errorf("can not get side: %w", err)
		}

		price, err := cmd.Flags().GetString("price")
		if err != nil {
			return fmt.Errorf("can not get price: %w", err)
		}

		asMarketOrder, err := cmd.Flags().GetBool("market")
		if err != nil {
			return err
		}

		quantity, err := cmd.Flags().GetString("quantity")
		if err != nil {
			return fmt.Errorf("can not get quantity: %w", err)
		}

		marginOrderSideEffect, err := cmd.Flags().GetString("margin-side-effect")
		if err != nil {
			return fmt.Errorf("can not get quantity: %w", err)
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

		market, ok := session.Market(symbol)
		if !ok {
			return fmt.Errorf("market definition %s not found", symbol)
		}

		so := types.SubmitOrder{
			Symbol:           symbol,
			Side:             types.SideType(strings.ToUpper(side)),
			Type:             types.OrderTypeLimit,
			Quantity:         fixedpoint.MustNewFromString(quantity),
			Market:           market,
			MarginSideEffect: types.MarginOrderSideEffectType(marginOrderSideEffect),
		}

		if asMarketOrder {
			so.Type = types.OrderTypeMarket
			so.Price = fixedpoint.Zero
		} else {
			if len(price) == 0 {
				return fmt.Errorf("price is required for limit order submission")
			}

			so.Type = types.OrderTypeLimit
			so.Price = fixedpoint.MustNewFromString(price)
			so.TimeInForce = types.TimeInForceGTC
		}

		co, err := session.Exchange.SubmitOrder(ctx, so)
		if err != nil {
			return err
		}

		log.Infof("submitted order: %+v\ncreated order: %+v", so, co)
		return nil
	},
}

func init() {
	listOrdersCmd.Flags().String("session", "", "the exchange session name for sync")
	listOrdersCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")

	getOrderCmd.Flags().String("session", "", "the exchange session name for sync")
	getOrderCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")
	getOrderCmd.Flags().String("order-id", "", "order id")

	submitOrderCmd.Flags().String("session", "", "the exchange session name for sync")
	submitOrderCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")
	submitOrderCmd.Flags().String("side", "", "the trading side: buy or sell")
	submitOrderCmd.Flags().String("price", "", "the trading price")
	submitOrderCmd.Flags().String("quantity", "", "the trading quantity")
	submitOrderCmd.Flags().Bool("market", false, "submit order as a market order")
	submitOrderCmd.Flags().String("margin-side-effect", "", "margin order side effect")

	executeOrderCmd.Flags().String("session", "", "the exchange session name for sync")
	executeOrderCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")
	executeOrderCmd.Flags().String("side", "", "the trading side: buy or sell")
	executeOrderCmd.Flags().String("target-quantity", "", "target quantity")
	executeOrderCmd.Flags().String("slice-quantity", "", "slice quantity")
	executeOrderCmd.Flags().String("stop-price", "0", "stop price")
	executeOrderCmd.Flags().Duration("update-interval", time.Second*10, "order update time")
	executeOrderCmd.Flags().Duration("deadline", 0, "deadline of the order execution")
	executeOrderCmd.Flags().Int("price-ticks", 0, "the number of price tick for the jump spread, default to 0")

	RootCmd.AddCommand(listOrdersCmd)
	RootCmd.AddCommand(getOrderCmd)
	RootCmd.AddCommand(submitOrderCmd)
	RootCmd.AddCommand(executeOrderCmd)
}

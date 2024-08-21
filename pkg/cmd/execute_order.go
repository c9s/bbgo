package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	_ "github.com/c9s/bbgo/pkg/twap"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"

	"github.com/c9s/bbgo/pkg/twap/v2"
)

func init() {
	executeOrderCmd.Flags().String("session", "", "the exchange session name for sync")
	executeOrderCmd.Flags().String("symbol", "", "the trading pair, like btcusdt")
	executeOrderCmd.Flags().String("side", "", "the trading side: buy or sell")
	executeOrderCmd.Flags().String("target-quantity", "", "target quantity")
	executeOrderCmd.Flags().String("slice-quantity", "", "slice quantity")
	executeOrderCmd.Flags().String("stop-price", "0", "stop price")
	executeOrderCmd.Flags().String("order-update-rate-limit", "1s", "order update rate limit, syntax: 1+1/1m")
	executeOrderCmd.Flags().Duration("update-interval", time.Second*10, "order update time")
	executeOrderCmd.Flags().Duration("delay-interval", time.Second*3, "order delay time after filled")
	executeOrderCmd.Flags().Duration("deadline", 0, "deadline duration of the order execution, e.g. 1h")
	executeOrderCmd.Flags().Int("price-ticks", 0, "the number of price tick for the jump spread, default to 0")
	RootCmd.AddCommand(executeOrderCmd)
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

		orderUpdateRateLimitStr, err := cmd.Flags().GetString("order-update-rate-limit")
		if err != nil {
			return err
		}

		updateInterval, err := cmd.Flags().GetDuration("update-interval")
		if err != nil {
			return err
		}

		delayInterval, err := cmd.Flags().GetDuration("delay-interval")
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

		market, ok := session.Market(symbol)
		if !ok {
			return fmt.Errorf("market %s not found", symbol)
		}

		executor := twap.NewFixedQuantityExecutor(session.Exchange, symbol, market, side, targetQuantity, sliceQuantity)

		if updateInterval > 0 {
			executor.SetUpdateInterval(updateInterval)
		}

		if len(orderUpdateRateLimitStr) > 0 {
			rateLimit, err := util.ParseRateLimitSyntax(orderUpdateRateLimitStr)
			if err != nil {
				return err
			}

			executor.SetOrderUpdateRateLimit(rateLimit)
		}

		if delayInterval > 0 {
			executor.SetDelayInterval(delayInterval)
		}

		if stopPrice.Sign() > 0 {
			executor.SetStopPrice(stopPrice)
		}

		// NumOfTicks:     numOfPriceTicks,
		if !deadlineTime.IsZero() {
			executor.SetDeadlineTime(deadlineTime)
		}

		if numOfPriceTicks > 0 {
			executor.SetNumOfTicks(numOfPriceTicks)
		}

		if err := executor.Start(executionCtx); err != nil {
			return err
		}

		var sigC = make(chan os.Signal, 1)
		signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigC)

		select {
		case <-ctx.Done():

		case sig := <-sigC:
			logrus.Warnf("signal %v", sig)
			logrus.Infof("shutting down order executor...")
			shutdownCtx, cancelShutdown := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
			executor.Shutdown(shutdownCtx)
			cancelShutdown()

		case <-executor.Done():
			logrus.Infof("the order execution is completed")

		}

		return nil
	},
}

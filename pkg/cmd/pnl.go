package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	PnLCmd.Flags().StringArray("session", []string{}, "target exchange sessions")
	PnLCmd.Flags().String("symbol", "", "trading symbol")
	PnLCmd.Flags().Bool("include-transfer", false, "convert transfer records into trades")
	PnLCmd.Flags().Bool("sync", false, "sync before loading trades")
	PnLCmd.Flags().String("since", "", "query trades from a time point")
	PnLCmd.Flags().Uint64("limit", 0, "number of trades")
	RootCmd.AddCommand(PnLCmd)
}

var PnLCmd = &cobra.Command{
	Use:          "pnl",
	Short:        "Average Cost Based PnL Calculator",
	Long:         "This command calculates the average cost-based profit from your total trades",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		sessionNames, err := cmd.Flags().GetStringArray("session")
		if err != nil {
			return err
		}

		if len(sessionNames) == 0 {
			return errors.New("--session [SESSION] is required")
		}

		wantSync, err := cmd.Flags().GetBool("sync")
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		if len(symbol) == 0 {
			return errors.New("--symbol [SYMBOL] is required")
		}

		// this is the default since
		since := time.Now().AddDate(-1, 0, 0)

		sinceOpt, err := cmd.Flags().GetString("since")
		if err != nil {
			return err
		}

		if sinceOpt != "" {
			lt, err := types.ParseLooseFormatTime(sinceOpt)
			if err != nil {
				return err
			}
			since = lt.Time()
		}

		until := time.Now()

		includeTransfer, err := cmd.Flags().GetBool("include-transfer")
		if err != nil {
			return err
		}

		limit, err := cmd.Flags().GetUint64("limit")
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()

		if err := environ.ConfigureDatabase(ctx); err != nil {
			return err
		}

		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		for _, sessionName := range sessionNames {
			session, ok := environ.Session(sessionName)
			if !ok {
				return fmt.Errorf("session %s not found", sessionName)
			}

			if wantSync {
				if err := environ.SyncSession(ctx, session, symbol); err != nil {
					return err
				}
			}

			if includeTransfer {
				exchange := session.Exchange
				market, _ := session.Market(symbol)
				transferService, ok := exchange.(types.ExchangeTransferService)
				if !ok {
					return fmt.Errorf("session exchange %s does not implement transfer service", sessionName)
				}

				deposits, err := transferService.QueryDepositHistory(ctx, market.BaseCurrency, since, until)
				if err != nil {
					return err
				}
				_ = deposits

				withdrawals, err := transferService.QueryWithdrawHistory(ctx, market.BaseCurrency, since, until)
				if err != nil {
					return err
				}

				sort.Slice(withdrawals, func(i, j int) bool {
					a := withdrawals[i].ApplyTime.Time()
					b := withdrawals[j].ApplyTime.Time()
					return a.Before(b)
				})

				// we need the backtest klines for the daily prices
				backtestService := &service.BacktestService{DB: environ.DatabaseService.DB}
				if err := backtestService.Sync(ctx, exchange, symbol, types.Interval1d, since, until); err != nil {
					return err
				}
			}
		}

		if err = environ.Init(ctx); err != nil {
			return err
		}

		session, _ := environ.Session(sessionNames[0])
		exchange := session.Exchange

		var trades []types.Trade
		tradingFeeCurrency := exchange.PlatformFeeCurrency()
		if strings.HasPrefix(symbol, tradingFeeCurrency) {
			log.Infof("loading all trading fee currency related trades: %s", symbol)
			trades, err = environ.TradeService.QueryForTradingFeeCurrency(exchange.Name(), symbol, tradingFeeCurrency)
		} else {
			trades, err = environ.TradeService.Query(service.QueryTradesOptions{
				Symbol:   symbol,
				Limit:    limit,
				Sessions: sessionNames,
				Since:    &since,
			})
		}

		if err != nil {
			return err
		}

		if len(trades) == 0 {
			return errors.New("empty trades, you need to run sync command to sync the trades from the exchange first")
		}

		trades = types.SortTradesAscending(trades)

		log.Infof("%d trades loaded", len(trades))

		tickers, err := exchange.QueryTickers(ctx, symbol)
		if err != nil {
			return err
		}

		currentTick, ok := tickers[symbol]
		if !ok {
			return errors.New("no ticker data for current price")
		}

		market, ok := session.Market(symbol)
		if !ok {
			return fmt.Errorf("market not found: %s, %s", symbol, session.Exchange.Name())
		}

		currentPrice := currentTick.Last
		calculator := &pnl.AverageCostCalculator{
			TradingFeeCurrency: tradingFeeCurrency,
			Market:             market,
		}

		report := calculator.Calculate(symbol, trades, currentPrice)
		report.Print()

		log.Warnf("note that if you're using cross-exchange arbitrage, the PnL won't be accurate")
		log.Warnf("withdrawal and deposits are not considered in the PnL")
		return nil
	},
}

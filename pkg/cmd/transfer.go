package cmd

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	TransferHistoryCmd.Flags().String("session", "", "target exchange session")
	TransferHistoryCmd.Flags().String("asset", "", "trading symbol")
	TransferHistoryCmd.Flags().String("since", "", "since time")
	RootCmd.AddCommand(TransferHistoryCmd)
}

type timeRecord struct {
	Record interface{}
	Time   time.Time
}

type timeSlice []timeRecord

func (p timeSlice) Len() int {
	return len(p)
}

func (p timeSlice) Less(i, j int) bool {
	return p[i].Time.Before(p[j].Time)
}

func (p timeSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

var TransferHistoryCmd = &cobra.Command{
	Use:   "transfer-history",
	Short: "show transfer history",

	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		userConfig, err := bbgo.Load(configFile, false)
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()
		if err := bbgo.BootstrapEnvironment(ctx, environ, userConfig); err != nil {
			return err
		}

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		asset, err := cmd.Flags().GetString("asset")
		if err != nil {
			return err
		}

		session, ok := environ.Session(sessionName)
		if !ok {
			return fmt.Errorf("session %s not found", sessionName)
		}

		// default
		var now = time.Now()
		var since = now.AddDate(-1, 0, 0)
		var until = now

		sinceStr, err := cmd.Flags().GetString("since")
		if err != nil {
			return err
		}

		if len(sinceStr) > 0 {
			loc, err := time.LoadLocation("Asia/Taipei")
			if err != nil {
				return err
			}

			since, err = time.ParseInLocation("2006-01-02", sinceStr, loc)
			if err != nil {
				return err
			}
		}

		var records timeSlice

		exchange, ok := session.Exchange.(types.ExchangeTransferService)
		if !ok {
			return fmt.Errorf("exchange session %s does not implement transfer service", sessionName)
		}

		deposits, err := exchange.QueryDepositHistory(ctx, asset, since, until)
		if err != nil {
			return err
		}
		for _, d := range deposits {
			records = append(records, timeRecord{
				Record: d,
				Time:   d.EffectiveTime(),
			})
		}

		withdraws, err := exchange.QueryWithdrawHistory(ctx, asset, since, until)
		if err != nil {
			return err
		}
		for _, w := range withdraws {
			records = append(records, timeRecord{
				Record: w,
				Time:   w.EffectiveTime(),
			})
		}

		sort.Sort(records)

		for _, record := range records {
			switch record := record.Record.(type) {

			case types.Deposit:
				logrus.Infof("%s: <--- DEPOSIT %v %s [%s]", record.Time, record.Amount, record.Asset, record.Status)

			case types.Withdraw:
				logrus.Infof("%s: ---> WITHDRAW %v %s  [%s]", record.ApplyTime, record.Amount, record.Asset, record.Status)

			default:
				logrus.Infof("unknown record: %+v", record)

			}
		}

		stats := calBaselineStats(asset, deposits, withdraws)
		for asset, quantity := range stats.TotalDeposit {
			logrus.Infof("total %s deposit: %v", asset, quantity)
		}

		for asset, quantity := range stats.TotalWithdraw {
			logrus.Infof("total %s withdraw: %v", asset, quantity)
		}

		for asset, quantity := range stats.BaselineBalance {
			logrus.Infof("baseline %s balance: %v", asset, quantity)
		}

		return nil
	},
}

type BaselineStats struct {
	Asset           string
	TotalDeposit    map[string]fixedpoint.Value
	TotalWithdraw   map[string]fixedpoint.Value
	BaselineBalance map[string]fixedpoint.Value
}

func calBaselineStats(asset string, deposits []types.Deposit, withdraws []types.Withdraw) (stats BaselineStats) {
	stats.Asset = asset
	stats.TotalDeposit = make(map[string]fixedpoint.Value)
	stats.TotalWithdraw = make(map[string]fixedpoint.Value)
	stats.BaselineBalance = make(map[string]fixedpoint.Value)

	for _, deposit := range deposits {
		if deposit.Status == types.DepositSuccess {
			if _, ok := stats.TotalDeposit[deposit.Asset]; !ok {
				stats.TotalDeposit[deposit.Asset] = fixedpoint.Zero
			}

			stats.TotalDeposit[deposit.Asset] = stats.TotalDeposit[deposit.Asset].Add(deposit.Amount)
		}
	}

	for _, withdraw := range withdraws {
		if withdraw.Status == "completed" {
			if _, ok := stats.TotalWithdraw[withdraw.Asset]; !ok {
				stats.TotalWithdraw[withdraw.Asset] = fixedpoint.Zero
			}

			stats.TotalWithdraw[withdraw.Asset] = stats.TotalWithdraw[withdraw.Asset].Add(withdraw.Amount)
		}
	}

	for asset, deposit := range stats.TotalDeposit {
		withdraw, ok := stats.TotalWithdraw[asset]
		if !ok {
			withdraw = fixedpoint.Zero
		}

		stats.BaselineBalance[asset] = deposit.Sub(withdraw)
	}

	return stats
}

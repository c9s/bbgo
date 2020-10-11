package cmd

import (
	"context"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	transferHistoryCmd.Flags().String("exchange", "", "target exchange")
	transferHistoryCmd.Flags().String("asset", "BTC", "trading symbol")
	transferHistoryCmd.Flags().String("since", "", "since time")
	RootCmd.AddCommand(transferHistoryCmd)
}



type TimeRecord struct {
	Record interface{}
	Time time.Time
}

type timeSlice []TimeRecord

func (p timeSlice) Len() int {
	return len(p)
}

func (p timeSlice) Less(i, j int) bool {
	return p[i].Time.Before(p[j].Time)
}

func (p timeSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}





var transferHistoryCmd = &cobra.Command{
	Use:          "transfer-history",
	Short:        "show transfer history",

	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		_ = ctx

		exchangeNameStr, err := cmd.Flags().GetString("exchange")
		if err != nil {
			return err
		}

		exchangeName, err := types.ValidExchangeName(exchangeNameStr)
		if err != nil {
			return err
		}

		asset, err := cmd.Flags().GetString("asset")
		if err != nil {
			return err
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


		exchange := newExchange(exchangeName)

		var records timeSlice

		deposits, err := exchange.QueryDepositHistory(ctx, asset, since, until)
		if err != nil {
			return err
		}
		for _, d := range deposits {
			records = append(records, TimeRecord{
				Record: d,
				Time:   d.EffectiveTime(),
			})
		}

		withdraws, err := exchange.QueryWithdrawHistory(ctx, asset, since, until)
		if err != nil {
			return err
		}
		for _, w := range withdraws {
			records = append(records, TimeRecord{
				Record: w,
				Time:   w.EffectiveTime(),
			})
		}

		sort.Sort(records)

		for _, record := range records {
			switch record := record.Record.(type) {

			case types.Deposit:
				log.Infof("%s: %s <== (deposit) %f [%s]", record.Time, record.Asset, record.Amount, record.Status)

			case types.Withdraw:
				log.Infof("%s: %s ==> (withdraw) %f [%s]", record.ApplyTime, record.Asset, record.Amount, record.Status)

			default:
				log.Infof("unknown record: %+v", record)

			}
		}

		stats := calBaselineStats(asset, deposits, withdraws)
		log.Infof("total %s deposit: %f (x %d)", asset, stats.TotalDeposit, stats.NumOfDeposit)
		log.Infof("total %s withdraw: %f (x %d)", asset, stats.TotalWithdraw, stats.NumOfWithdraw)
		log.Infof("baseline %s balance: %f", asset, stats.BaselineBalance)
		return nil
	},
}

type BaselineStats struct {
	Asset string
	NumOfDeposit int
	NumOfWithdraw int
	TotalDeposit float64
	TotalWithdraw float64
	BaselineBalance float64
}

func calBaselineStats(asset string, deposits []types.Deposit, withdraws []types.Withdraw) (stats BaselineStats) {
	stats.Asset = asset
	stats.NumOfDeposit = len(deposits)
	stats.NumOfWithdraw = len(withdraws)

	for _, deposit := range deposits {
		if deposit.Status == types.DepositSuccess {
			stats.TotalDeposit += deposit.Amount
		}
	}

	for _, withdraw := range withdraws {
		if withdraw.Status == "completed" {
			stats.TotalWithdraw += withdraw.Amount
		}
	}

	stats.BaselineBalance = stats.TotalDeposit - stats.TotalWithdraw
	return stats
}

package report

import (
	"fmt"
	"github.com/c9s/bbgo/pkg/data/tsv"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
	"strconv"
)

// AccumulatedProfitReport For accumulated profit report output
type AccumulatedProfitReport struct {
	// ProfitMAWindow Accumulated profit SMA window
	ProfitMAWindow int `json:"profitMAWindow"`

	// ShortTermProfitWindow The window to sum up the short-term profit
	ShortTermProfitWindow int `json:"shortTermProfitWindow"`

	// TsvReportPath The path to output report to
	TsvReportPath string `json:"tsvReportPath"`

	symbol string

	types.IntervalWindow

	// ProfitMAWindow Accumulated profit SMA window
	AccumulateTradeWindow int `json:"accumulateTradeWindow"`

	// Accumulated profit
	accumulatedProfit            fixedpoint.Value
	accumulatedProfitPerInterval *types.Float64Series

	// Accumulated profit MA
	profitMA            *indicatorv2.SMAStream
	profitMAPerInterval floats.Slice

	// Profit of each interval
	ProfitPerInterval floats.Slice

	// Accumulated fee
	accumulatedFee            fixedpoint.Value
	accumulatedFeePerInterval floats.Slice

	// Win ratio
	winRatioPerInterval floats.Slice

	// Profit factor
	profitFactorPerInterval floats.Slice

	// Trade number
	accumulatedTrades            int
	accumulatedTradesPerInterval floats.Slice

	// Extra values
	strategyParameters [][2]string
}

func (r *AccumulatedProfitReport) Initialize(symbol string, interval types.Interval, window int) {
	r.symbol = symbol
	r.Interval = interval
	r.Window = window

	if r.ProfitMAWindow <= 0 {
		r.ProfitMAWindow = 60
	}

	if r.Window <= 0 {
		r.Window = 7
	}

	if r.ShortTermProfitWindow <= 0 {
		r.ShortTermProfitWindow = 7
	}

	r.accumulatedProfitPerInterval = types.NewFloat64Series()
	r.profitMA = indicatorv2.SMA(r.accumulatedProfitPerInterval, r.ProfitMAWindow)
}

func (r *AccumulatedProfitReport) AddStrategyParameter(title string, value string) {
	r.strategyParameters = append(r.strategyParameters, [2]string{title, value})
}

func (r *AccumulatedProfitReport) AddTrade(trade types.Trade) {
	r.accumulatedFee = r.accumulatedFee.Add(trade.Fee)
	r.accumulatedTrades += 1
}

func (r *AccumulatedProfitReport) Rotate(ps *types.ProfitStats, ts *types.TradeStats) {
	// Accumulated profit
	r.accumulatedProfit = r.accumulatedProfit.Add(ps.AccumulatedNetProfit)
	r.accumulatedProfitPerInterval.PushAndEmit(r.accumulatedProfit.Float64())

	// Profit of each interval
	r.ProfitPerInterval.Update(ps.AccumulatedNetProfit.Float64())

	// Profit MA
	r.profitMAPerInterval.Update(r.profitMA.Last(0))

	// Accumulated Fee
	r.accumulatedFeePerInterval.Update(r.accumulatedFee.Float64())

	// Trades
	r.accumulatedTradesPerInterval.Update(float64(r.accumulatedTrades))

	// Win ratio
	r.winRatioPerInterval.Update(ts.WinningRatio.Float64())

	// Profit factor
	r.profitFactorPerInterval.Update(ts.ProfitFactor.Float64())
}

// CsvHeader returns a header slice
func (r *AccumulatedProfitReport) CsvHeader() []string {
	titles := []string{
		"#",
		"Symbol",
		"Total Net Profit",
		fmt.Sprintf("Total Net Profit %sMA%d", r.Interval, r.ProfitMAWindow),
		fmt.Sprintf("%s%d Net Profit", r.Interval, r.ShortTermProfitWindow),
		"accumulatedFee",
		"winRatio",
		"profitFactor",
		fmt.Sprintf("%s%d Trades", r.Interval, r.AccumulateTradeWindow),
	}

	for i := 0; i < len(r.strategyParameters); i++ {
		titles = append(titles, r.strategyParameters[i][0])
	}

	return titles
}

// CsvRecords returns a data slice
func (r *AccumulatedProfitReport) CsvRecords() [][]string {
	var data [][]string

	for i := 0; i <= r.Window-1; i++ {
		values := []string{
			strconv.Itoa(i + 1),
			r.symbol,
			strconv.FormatFloat(r.accumulatedProfitPerInterval.Last(i), 'f', 4, 64),
			strconv.FormatFloat(r.profitMAPerInterval.Last(i), 'f', 4, 64),
			strconv.FormatFloat(r.accumulatedProfitPerInterval.Last(i)-r.accumulatedProfitPerInterval.Last(i+r.ShortTermProfitWindow), 'f', 4, 64),
			strconv.FormatFloat(r.accumulatedFeePerInterval.Last(i), 'f', 4, 64),
			strconv.FormatFloat(r.winRatioPerInterval.Last(i), 'f', 4, 64),
			strconv.FormatFloat(r.profitFactorPerInterval.Last(i), 'f', 4, 64),
			strconv.FormatFloat(r.accumulatedTradesPerInterval.Last(i)-r.accumulatedTradesPerInterval.Last(i+r.AccumulateTradeWindow), 'f', 4, 64),
		}
		for j := 0; j < len(r.strategyParameters); j++ {
			values = append(values, r.strategyParameters[j][1])
		}

		data = append(data, values)
	}

	return data
}

// Output Accumulated profit report to a TSV file
func (r *AccumulatedProfitReport) Output() {
	if r.TsvReportPath != "" {
		// Open specified file for appending
		tsvwiter, err := tsv.AppendWriterFile(r.TsvReportPath)
		if err != nil {
			panic(err)
		}
		defer tsvwiter.Close()

		// Column Title
		_ = tsvwiter.Write(r.CsvHeader())

		// Output data rows
		_ = tsvwiter.WriteAll(r.CsvRecords())
	}
}

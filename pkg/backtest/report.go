package backtest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gofrs/flock"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

type Run struct {
	ID     string       `json:"id"`
	Config *bbgo.Config `json:"config"`
	Time   time.Time    `json:"time"`
}

type ReportIndex struct {
	Runs []Run `json:"runs,omitempty"`
}

// SummaryReport is the summary of the back-test session
type SummaryReport struct {
	StartTime            time.Time        `json:"startTime"`
	EndTime              time.Time        `json:"endTime"`
	Sessions             []string         `json:"sessions"`
	Symbols              []string         `json:"symbols"`
	Intervals            []types.Interval `json:"intervals"`
	InitialTotalBalances types.BalanceMap `json:"initialTotalBalances"`
	FinalTotalBalances   types.BalanceMap `json:"finalTotalBalances"`

	InitialEquityValue fixedpoint.Value `json:"initialEquityValue"`
	FinalEquityValue   fixedpoint.Value `json:"finalEquityValue"`

	// TotalProfit is the profit aggregated from the symbol reports
	TotalProfit           fixedpoint.Value `json:"totalProfit,omitempty"`
	TotalUnrealizedProfit fixedpoint.Value `json:"totalUnrealizedProfit,omitempty"`

	TotalGrossProfit fixedpoint.Value `json:"totalGrossProfit,omitempty"`
	TotalGrossLoss   fixedpoint.Value `json:"totalGrossLoss,omitempty"`

	SymbolReports []SessionSymbolReport `json:"symbolReports,omitempty"`

	Manifests Manifests `json:"manifests,omitempty"`
}

func ReadSummaryReport(filename string) (*SummaryReport, error) {
	o, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var report SummaryReport
	err = json.Unmarshal(o, &report)
	return &report, err
}

// SessionSymbolReport is the report per exchange session
// trades are merged, collected and re-calculated
type SessionSymbolReport struct {
	Exchange        types.ExchangeName        `json:"exchange"`
	Symbol          string                    `json:"symbol,omitempty"`
	Intervals       []types.Interval          `json:"intervals,omitempty"`
	Subscriptions   []types.Subscription      `json:"subscriptions"`
	Market          types.Market              `json:"market"`
	LastPrice       fixedpoint.Value          `json:"lastPrice,omitempty"`
	StartPrice      fixedpoint.Value          `json:"startPrice,omitempty"`
	PnL             *pnl.AverageCostPnlReport `json:"pnl,omitempty"`
	InitialBalances types.BalanceMap          `json:"initialBalances,omitempty"`
	FinalBalances   types.BalanceMap          `json:"finalBalances,omitempty"`
	Manifests       Manifests                 `json:"manifests,omitempty"`
}

func (r *SessionSymbolReport) InitialEquityValue() fixedpoint.Value {
	return InQuoteAsset(r.InitialBalances, r.Market, r.StartPrice)
}

func (r *SessionSymbolReport) FinalEquityValue() fixedpoint.Value {
	return InQuoteAsset(r.FinalBalances, r.Market, r.LastPrice)
}

func (r *SessionSymbolReport) Print(wantBaseAssetBaseline bool) {
	color.Green("%s %s PROFIT AND LOSS REPORT", r.Exchange, r.Symbol)
	color.Green("===============================================")
	r.PnL.Print()

	initQuoteAsset := r.InitialEquityValue()
	finalQuoteAsset := r.FinalEquityValue()
	color.Green("INITIAL ASSET IN %s ~= %s %s (1 %s = %v)", r.Market.QuoteCurrency, r.Market.FormatQuantity(initQuoteAsset), r.Market.QuoteCurrency, r.Market.BaseCurrency, r.StartPrice)
	color.Green("FINAL ASSET IN %s ~= %s %s (1 %s = %v)", r.Market.QuoteCurrency, r.Market.FormatQuantity(finalQuoteAsset), r.Market.QuoteCurrency, r.Market.BaseCurrency, r.LastPrice)

	if r.PnL.Profit.Sign() > 0 {
		color.Green("REALIZED PROFIT: +%v %s", r.PnL.Profit, r.Market.QuoteCurrency)
	} else {
		color.Red("REALIZED PROFIT: %v %s", r.PnL.Profit, r.Market.QuoteCurrency)
	}

	if r.PnL.UnrealizedProfit.Sign() > 0 {
		color.Green("UNREALIZED PROFIT: +%v %s", r.PnL.UnrealizedProfit, r.Market.QuoteCurrency)
	} else {
		color.Red("UNREALIZED PROFIT: %v %s", r.PnL.UnrealizedProfit, r.Market.QuoteCurrency)
	}

	if finalQuoteAsset.Compare(initQuoteAsset) > 0 {
		color.Green("ASSET INCREASED: +%v %s (+%s)", finalQuoteAsset.Sub(initQuoteAsset), r.Market.QuoteCurrency, finalQuoteAsset.Sub(initQuoteAsset).Div(initQuoteAsset).FormatPercentage(2))
	} else {
		color.Red("ASSET DECREASED: %v %s (%s)", finalQuoteAsset.Sub(initQuoteAsset), r.Market.QuoteCurrency, finalQuoteAsset.Sub(initQuoteAsset).Div(initQuoteAsset).FormatPercentage(2))
	}

	if wantBaseAssetBaseline {
		if r.LastPrice.Compare(r.StartPrice) > 0 {
			color.Green("%s BASE ASSET PERFORMANCE: +%s (= (%s - %s) / %s)",
				r.Market.BaseCurrency,
				r.LastPrice.Sub(r.StartPrice).Div(r.StartPrice).FormatPercentage(2),
				r.LastPrice.FormatString(2),
				r.StartPrice.FormatString(2),
				r.StartPrice.FormatString(2))
		} else {
			color.Red("%s BASE ASSET PERFORMANCE: %s (= (%s - %s) / %s)",
				r.Market.BaseCurrency,
				r.LastPrice.Sub(r.StartPrice).Div(r.StartPrice).FormatPercentage(2),
				r.LastPrice.FormatString(2),
				r.StartPrice.FormatString(2),
				r.StartPrice.FormatString(2))
		}
	}
}

const SessionTimeFormat = "2006-01-02T15_04"

// FormatSessionName returns the back-test session name
func FormatSessionName(sessions []string, symbols []string, startTime, endTime time.Time) string {
	return fmt.Sprintf("%s_%s_%s-%s",
		strings.Join(sessions, "-"),
		strings.Join(symbols, "-"),
		startTime.Format(SessionTimeFormat),
		endTime.Format(SessionTimeFormat),
	)
}

func WriteReportIndex(outputDirectory string, reportIndex *ReportIndex) error {
	indexFile := filepath.Join(outputDirectory, "index.json")
	if err := util.WriteJsonFile(indexFile, reportIndex); err != nil {
		return err
	}
	return nil
}

func LoadReportIndex(outputDirectory string) (*ReportIndex, error) {
	var reportIndex ReportIndex
	indexFile := filepath.Join(outputDirectory, "index.json")
	if _, err := os.Stat(indexFile); err == nil {
		o, err := ioutil.ReadFile(indexFile)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(o, &reportIndex); err != nil {
			return nil, err
		}
	}

	return &reportIndex, nil
}

func AddReportIndexRun(outputDirectory string, run Run) error {
	// append report index
	lockFile := filepath.Join(outputDirectory, ".report.lock")
	fileLock := flock.New(lockFile)

	err := fileLock.Lock()
	if err != nil {
		return err
	}

	defer func() {
		if err := fileLock.Unlock(); err != nil {
			log.WithError(err).Errorf("report index file lock error: %s", lockFile)
		}
		if err := os.Remove(lockFile); err != nil {
			log.WithError(err).Errorf("can not remove lock file: %s", lockFile)
		}
	}()
	reportIndex, err := LoadReportIndex(outputDirectory)
	if err != nil {
		return err
	}

	reportIndex.Runs = append(reportIndex.Runs, run)
	return WriteReportIndex(outputDirectory, reportIndex)
}

// InQuoteAsset converts all balances in quote asset
func InQuoteAsset(balances types.BalanceMap, market types.Market, price fixedpoint.Value) fixedpoint.Value {
	quote := balances[market.QuoteCurrency]
	base := balances[market.BaseCurrency]
	return base.Total().Mul(price).Add(quote.Total())
}

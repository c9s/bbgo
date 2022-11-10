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
	PnL             *pnl.AverageCostPnLReport `json:"pnl,omitempty"`
	InitialBalances types.BalanceMap          `json:"initialBalances,omitempty"`
	FinalBalances   types.BalanceMap          `json:"finalBalances,omitempty"`
	Manifests       Manifests                 `json:"manifests,omitempty"`
	Sharpe          fixedpoint.Value          `json:"sharpeRatio"`
	Sortino         fixedpoint.Value          `json:"sortinoRatio"`
	ProfitFactor    fixedpoint.Value          `json:"profitFactor"`
	WinningRatio    fixedpoint.Value          `json:"winningRatio"`
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

	if r.Sharpe.Sign() > 0 {
		color.Green("REALIZED SHARPE RATIO: %s", r.Sharpe.FormatString(4))
	} else {
		color.Red("REALIZED SHARPE RATIO: %s", r.Sharpe.FormatString(4))
	}

	if r.Sortino.Sign() > 0 {
		color.Green("REALIZED SORTINO RATIO: %s", r.Sortino.FormatString(4))
	} else {
		color.Red("REALIZED SORTINO RATIO: %s", r.Sortino.FormatString(4))
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
	indexFile := getReportIndexPath(outputDirectory)
	indexLock := flock.New(indexFile)

	if err := indexLock.Lock(); err != nil {
		log.WithError(err).Errorf("report index file lock error while write report: %s", err)
		return err
	}
	defer func() {
		if err := indexLock.Unlock(); err != nil {
			log.WithError(err).Errorf("report index file unlock error while write report: %s", err)
		}
	}()

	return writeReportIndexLocked(outputDirectory, reportIndex)
}

func LoadReportIndex(outputDirectory string) (*ReportIndex, error) {
	indexFile := getReportIndexPath(outputDirectory)
	indexLock := flock.New(indexFile)

	if err := indexLock.Lock(); err != nil {
		log.WithError(err).Errorf("report index file lock error while load report: %s", err)
		return nil, err
	}
	defer func() {
		if err := indexLock.Unlock(); err != nil {
			log.WithError(err).Errorf("report index file unlock error while load report: %s", err)
		}
	}()

	return loadReportIndexLocked(indexFile)
}

func AddReportIndexRun(outputDirectory string, run Run) error {
	// append report index
	indexFile := getReportIndexPath(outputDirectory)
	indexLock := flock.New(indexFile)

	if err := indexLock.Lock(); err != nil {
		log.WithError(err).Errorf("report index file lock error: %s", err)
		return err
	}
	defer func() {
		if err := indexLock.Unlock(); err != nil {
			log.WithError(err).Errorf("report index file unlock error: %s", err)
		}
	}()

	reportIndex, err := loadReportIndexLocked(indexFile)
	if err != nil {
		return err
	}

	reportIndex.Runs = append(reportIndex.Runs, run)
	return writeReportIndexLocked(indexFile, reportIndex)
}

// InQuoteAsset converts all balances in quote asset
func InQuoteAsset(balances types.BalanceMap, market types.Market, price fixedpoint.Value) fixedpoint.Value {
	quote := balances[market.QuoteCurrency]
	base := balances[market.BaseCurrency]
	return base.Total().Mul(price).Add(quote.Total())
}

func getReportIndexPath(outputDirectory string) string {
	return filepath.Join(outputDirectory, "index.json")
}

// writeReportIndexLocked must be protected by file lock
func writeReportIndexLocked(indexFilePath string, reportIndex *ReportIndex) error {
	if err := util.WriteJsonFile(indexFilePath, reportIndex); err != nil {
		return err
	}
	return nil
}

// loadReportIndexLocked must be protected by file lock
func loadReportIndexLocked(indexFilePath string) (*ReportIndex, error) {
	var reportIndex ReportIndex
	if fileInfo, err := os.Stat(indexFilePath); err != nil {
		return nil, err
	} else if fileInfo.Size() != 0 {
		o, err := ioutil.ReadFile(indexFilePath)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(o, &reportIndex); err != nil {
			return nil, err
		}
	}

	return &reportIndex, nil
}

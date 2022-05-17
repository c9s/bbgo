package backtest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	InitialTotalBalances types.BalanceMap `json:"initialTotalBalances"`
	FinalTotalBalances   types.BalanceMap `json:"finalTotalBalances"`
}

// SessionSymbolReport is the report per exchange session
// trades are merged, collected and re-calculated
type SessionSymbolReport struct {
	StartTime       time.Time                 `json:"startTime"`
	EndTime         time.Time                 `json:"endTime"`
	Symbol          string                    `json:"symbol,omitempty"`
	LastPrice       fixedpoint.Value          `json:"lastPrice,omitempty"`
	StartPrice      fixedpoint.Value          `json:"startPrice,omitempty"`
	PnLReport       *pnl.AverageCostPnlReport `json:"pnlReport,omitempty"`
	InitialBalances types.BalanceMap          `json:"initialBalances,omitempty"`
	FinalBalances   types.BalanceMap          `json:"finalBalances,omitempty"`
	Manifests       Manifests                 `json:"manifests,omitempty"`
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


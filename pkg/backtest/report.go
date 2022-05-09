package backtest

import (
	"fmt"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Report struct {
	StartTime       time.Time                 `json:"startTime"`
	EndTime         time.Time                 `json:"endTime"`
	Symbol          string                    `json:"symbol,omitempty"`
	LastPrice       fixedpoint.Value          `json:"lastPrice,omitempty"`
	StartPrice      fixedpoint.Value          `json:"startPrice,omitempty"`
	PnLReport       *pnl.AverageCostPnlReport `json:"pnlReport,omitempty"`
	InitialBalances types.BalanceMap          `json:"initialBalances,omitempty"`
	FinalBalances   types.BalanceMap          `json:"finalBalances,omitempty"`
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

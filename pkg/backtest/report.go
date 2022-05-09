package backtest

import (
	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Report struct {
	Symbol          string                    `json:"symbol,omitempty"`
	LastPrice       fixedpoint.Value          `json:"lastPrice,omitempty"`
	StartPrice      fixedpoint.Value          `json:"startPrice,omitempty"`
	PnLReport       *pnl.AverageCostPnlReport `json:"pnlReport,omitempty"`
	InitialBalances types.BalanceMap          `json:"initialBalances,omitempty"`
	FinalBalances   types.BalanceMap          `json:"finalBalances,omitempty"`
}


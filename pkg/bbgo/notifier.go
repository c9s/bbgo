package bbgo

import (
	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/types"
)

type Notifier interface {
	Notify(channel, format string, args ...interface{})
	NotifyTrade(trade *types.Trade)
	NotifyPnL(report *pnl.AverageCostPnlReport)
}

type NullNotifier struct{}

func (n *NullNotifier) Notify(format string, args ...interface{}) {
}

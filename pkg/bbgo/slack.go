package bbgo

import (
	"github.com/c9s/bbgo/pkg/accounting"
	"github.com/c9s/bbgo/pkg/types"
)

type Notifier interface {
	Notify(format string, args ...interface{})
	NotifyTrade(trade *types.Trade)
	NotifyPnL(report *accounting.ProfitAndLossReport)
}

type NullNotifier struct{}

func (n *NullNotifier) Notify(format string, args ...interface{}) {
}

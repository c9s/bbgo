package common

import (
	"sync"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	log "github.com/sirupsen/logrus"
)

type FeeBudget struct {
	DailyFeeBudgets map[string]fixedpoint.Value `json:"dailyFeeBudgets,omitempty"`
	DailyFeeTracker *util.DailyDataTracker      `persistence:"dailyFeeTracker"`

	mu sync.Mutex
}

func (f *FeeBudget) Initialize() {
	if f.DailyFeeTracker == nil {
		f.DailyFeeTracker = &util.DailyDataTracker{}
		f.DailyFeeTracker.Reset()
	}

	if f.DailyFeeTracker.IsOver24Hours() {
		log.Warn("[FeeBudget] state is over 24 hours, resetting to zero")
		f.DailyFeeTracker.Reset()
	}
}

func (f *FeeBudget) IsBudgetAllowed() bool {
	if f.DailyFeeBudgets == nil {
		return true
	}

	if f.DailyFeeTracker.Data == nil {
		return true
	}

	if f.DailyFeeTracker.IsOver24Hours() {
		f.DailyFeeTracker.Reset()
		return true
	}

	for asset, budget := range f.DailyFeeBudgets {
		if fee, ok := f.DailyFeeTracker.Data[asset]; ok {
			if fee.Compare(budget) >= 0 {
				log.Warnf("[FeeBudget] accumulative fee %s exceeded the fee budget %s, skipping...", fee.String(), budget.String())
				return false
			}
		}
	}

	return true
}

func (f *FeeBudget) HandleTradeUpdate(trade types.Trade) {
	log.Infof("[FeeBudget] received trade %s", trade.String())

	if f.DailyFeeTracker.IsOver24Hours() {
		f.DailyFeeTracker.Reset()
	}

	// safe check
	if f.DailyFeeTracker.Data == nil {
		f.mu.Lock()
		f.DailyFeeTracker.Data = make(map[string]fixedpoint.Value)
		f.mu.Unlock()
	}

	f.DailyFeeTracker.Data[trade.FeeCurrency] = f.DailyFeeTracker.Data[trade.FeeCurrency].Add(trade.Fee)
	log.Infof("[FeeBudget] accumulated fee: %s %s", f.DailyFeeTracker.Data[trade.FeeCurrency].String(), trade.FeeCurrency)
}

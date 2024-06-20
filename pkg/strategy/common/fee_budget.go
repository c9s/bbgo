package common

import (
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

type FeeBudget struct {
	DailyFeeBudgets map[string]fixedpoint.Value `json:"dailyFeeBudgets,omitempty"`
	State           *State                      `persistence:"state"`

	mu sync.Mutex
}

func (f *FeeBudget) Initialize() {
	if f.State == nil {
		f.State = &State{}
		f.State.Reset()
	}

	if f.State.IsOver24Hours() {
		log.Warn("[FeeBudget] state is over 24 hours, resetting to zero")
		f.State.Reset()
	}
}

func (f *FeeBudget) IsBudgetAllowed() bool {
	if f.DailyFeeBudgets == nil {
		return true
	}

	if f.State.AccumulatedFees == nil {
		return true
	}

	if f.State.IsOver24Hours() {
		f.State.Reset()
		return true
	}

	for asset, budget := range f.DailyFeeBudgets {
		if fee, ok := f.State.AccumulatedFees[asset]; ok {
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

	if f.State.IsOver24Hours() {
		f.State.Reset()
	}

	// safe check
	if f.State.AccumulatedFees == nil {
		f.mu.Lock()
		f.State.AccumulatedFees = make(map[string]fixedpoint.Value)
		f.mu.Unlock()
	}

	f.State.AccumulatedFees[trade.FeeCurrency] = f.State.AccumulatedFees[trade.FeeCurrency].Add(trade.Fee)
	log.Infof("[FeeBudget] accumulated fee: %s %s", f.State.AccumulatedFees[trade.FeeCurrency].String(), trade.FeeCurrency)
}

type State struct {
	AccumulatedFeeStartedAt time.Time                   `json:"accumulatedFeeStartedAt,omitempty"`
	AccumulatedFees         map[string]fixedpoint.Value `json:"accumulatedFees,omitempty"`
}

func (s *State) IsOver24Hours() bool {
	return time.Since(s.AccumulatedFeeStartedAt) >= 24*time.Hour
}

func (s *State) Reset() {
	t := time.Now()
	dateTime := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	log.Infof("[State] resetting accumulated started time to: %s", dateTime)

	s.AccumulatedFeeStartedAt = dateTime
	s.AccumulatedFees = make(map[string]fixedpoint.Value)
}

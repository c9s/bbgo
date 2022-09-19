package xmaker

import (
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type State struct {
	CoveredPosition fixedpoint.Value `json:"coveredPosition,omitempty"`

	// Deprecated:
	Position *types.Position `json:"position,omitempty"`

	// Deprecated:
	ProfitStats ProfitStats `json:"profitStats,omitempty"`
}

type ProfitStats struct {
	*types.ProfitStats

	lock sync.Mutex

	MakerExchange types.ExchangeName `json:"makerExchange"`

	AccumulatedMakerVolume    fixedpoint.Value `json:"accumulatedMakerVolume,omitempty"`
	AccumulatedMakerBidVolume fixedpoint.Value `json:"accumulatedMakerBidVolume,omitempty"`
	AccumulatedMakerAskVolume fixedpoint.Value `json:"accumulatedMakerAskVolume,omitempty"`

	TodayMakerVolume    fixedpoint.Value `json:"todayMakerVolume,omitempty"`
	TodayMakerBidVolume fixedpoint.Value `json:"todayMakerBidVolume,omitempty"`
	TodayMakerAskVolume fixedpoint.Value `json:"todayMakerAskVolume,omitempty"`
}

func (s *ProfitStats) AddTrade(trade types.Trade) {
	s.ProfitStats.AddTrade(trade)

	if trade.Exchange == s.MakerExchange {
		s.lock.Lock()
		s.AccumulatedMakerVolume = s.AccumulatedMakerVolume.Add(trade.Quantity)
		s.TodayMakerVolume = s.TodayMakerVolume.Add(trade.Quantity)

		switch trade.Side {

		case types.SideTypeSell:
			s.AccumulatedMakerAskVolume = s.AccumulatedMakerAskVolume.Add(trade.Quantity)
			s.TodayMakerAskVolume = s.TodayMakerAskVolume.Add(trade.Quantity)

		case types.SideTypeBuy:
			s.AccumulatedMakerBidVolume = s.AccumulatedMakerBidVolume.Add(trade.Quantity)
			s.TodayMakerBidVolume = s.TodayMakerBidVolume.Add(trade.Quantity)

		}
		s.lock.Unlock()
	}
}

func (s *ProfitStats) ResetToday() {
	s.ProfitStats.ResetToday(time.Now())

	s.lock.Lock()
	s.TodayMakerVolume = fixedpoint.Zero
	s.TodayMakerBidVolume = fixedpoint.Zero
	s.TodayMakerAskVolume = fixedpoint.Zero
	s.lock.Unlock()
}

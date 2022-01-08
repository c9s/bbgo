package xmaker

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type State struct {
	CoveredPosition fixedpoint.Value `json:"coveredPosition,omitempty"`
	Position        *types.Position  `json:"position,omitempty"`
	ProfitStats     ProfitStats      `json:"profitStats,omitempty"`
}

type ProfitStats struct {
	bbgo.ProfitStats

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
		s.AccumulatedMakerVolume.AtomicAdd(fixedpoint.NewFromFloat(trade.Quantity))
		s.TodayMakerVolume.AtomicAdd(fixedpoint.NewFromFloat(trade.Quantity))

		switch trade.Side {

		case types.SideTypeSell:
			s.AccumulatedMakerAskVolume.AtomicAdd(fixedpoint.NewFromFloat(trade.Quantity))
			s.TodayMakerAskVolume.AtomicAdd(fixedpoint.NewFromFloat(trade.Quantity))

		case types.SideTypeBuy:
			s.AccumulatedMakerBidVolume.AtomicAdd(fixedpoint.NewFromFloat(trade.Quantity))
			s.TodayMakerBidVolume.AtomicAdd(fixedpoint.NewFromFloat(trade.Quantity))

		}
	}
}

func (s *ProfitStats) ResetToday() {
	s.ProfitStats.ResetToday()
	s.TodayMakerVolume = 0
	s.TodayMakerBidVolume = 0
	s.TodayMakerAskVolume = 0
}

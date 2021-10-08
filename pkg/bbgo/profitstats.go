package bbgo

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"time"
)

type ProfitStats struct {
	AccumulatedPnL            fixedpoint.Value `json:"accumulatedPnL,omitempty"`
	AccumulatedNetProfit      fixedpoint.Value `json:"accumulatedNetProfit,omitempty"`
	AccumulatedProfit         fixedpoint.Value `json:"accumulatedProfit,omitempty"`
	AccumulatedLoss           fixedpoint.Value `json:"accumulatedLoss,omitempty"`
	AccumulatedSince          int64            `json:"accumulatedSince,omitempty"`

	TodayPnL            fixedpoint.Value `json:"todayPnL,omitempty"`
	TodayNetProfit      fixedpoint.Value `json:"todayNetProfit,omitempty"`
	TodayProfit         fixedpoint.Value `json:"todayProfit,omitempty"`
	TodayLoss           fixedpoint.Value `json:"todayLoss,omitempty"`
	TodaySince          int64            `json:"todaySince,omitempty"`
}

func (s *ProfitStats) AddProfit(profit, netProfit fixedpoint.Value) {
	s.AccumulatedPnL += profit
	s.AccumulatedNetProfit += netProfit

	s.TodayPnL += profit
	s.TodayNetProfit += netProfit

	if profit < 0 {
		s.AccumulatedLoss += profit
		s.TodayLoss += profit
	} else if profit > 0 {
		s.AccumulatedProfit += profit
		s.TodayProfit += profit
	}
}

func (s *ProfitStats) AddTrade(trade types.Trade) {
	if s.IsOver24Hours() {
		s.ResetToday()
	}
}

func (s *ProfitStats) IsOver24Hours() bool {
	return time.Since(time.Unix(s.TodaySince, 0)) > 24*time.Hour
}

func (s *ProfitStats) ResetToday() {
	s.TodayPnL = 0
	s.TodayNetProfit = 0
	s.TodayProfit = 0
	s.TodayLoss = 0

	var beginningOfTheDay = util.BeginningOfTheDay(time.Now().Local())
	s.TodaySince = beginningOfTheDay.Unix()
}

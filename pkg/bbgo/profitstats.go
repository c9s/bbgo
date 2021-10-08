package bbgo

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"time"
)

// Profit struct stores the PnL information
type Profit struct {
	// Profit is the profit of this trade made. negative profit means loss.
	Profit fixedpoint.Value `json:"profit" db:"profit"`

	// NetProfit is (profit - trading fee)
	NetProfit   fixedpoint.Value `json:"netProfit" db:"net_profit"`
	AverageCost fixedpoint.Value `json:"averageCost" db:"average_ost"`

	TradeAmount float64 `json:"tradeAmount" db:"trade_amount"`

	// ProfitMargin is a percentage of the profit and the capital
	ProfitMargin fixedpoint.Value `json:"profitMargin" db:"profit_margin"`

	QuoteCurrency string `json:"quote_currency" db:"quote_currency"`
	BaseCurrency  string `json:"base_currency" db:"base_currency"`

	// FeeInUSD is the summed fee of this profit,
	// you will need to convert the trade fee into USD since the fee currencies can be different.
	FeeInUSD           fixedpoint.Value `json:"feeInUSD" db:"fee_in_usd"`
	Time               time.Time        `json:"time" db:"time"`
	Strategy           string           `json:"strategy" db:"strategy"`
	StrategyInstanceID string           `json:"strategyInstanceID" db:"strategy_instance_id"`
}

type ProfitStats struct {
	AccumulatedPnL       fixedpoint.Value `json:"accumulatedPnL,omitempty"`
	AccumulatedNetProfit fixedpoint.Value `json:"accumulatedNetProfit,omitempty"`
	AccumulatedProfit    fixedpoint.Value `json:"accumulatedProfit,omitempty"`
	AccumulatedLoss      fixedpoint.Value `json:"accumulatedLoss,omitempty"`
	AccumulatedVolume    fixedpoint.Value `json:"accumulatedVolume,omitempty"`
	AccumulatedSince     int64            `json:"accumulatedSince,omitempty"`

	TodayPnL       fixedpoint.Value `json:"todayPnL,omitempty"`
	TodayNetProfit fixedpoint.Value `json:"todayNetProfit,omitempty"`
	TodayProfit    fixedpoint.Value `json:"todayProfit,omitempty"`
	TodayLoss      fixedpoint.Value `json:"todayLoss,omitempty"`
	TodaySince     int64            `json:"todaySince,omitempty"`
}

func (s *ProfitStats) AddProfit(profit Profit) {
	s.AccumulatedPnL += profit.Profit
	s.AccumulatedNetProfit += profit.NetProfit
	s.TodayPnL += profit.Profit
	s.TodayNetProfit += profit.NetProfit

	if profit.Profit < 0 {
		s.AccumulatedLoss += profit.Profit
		s.TodayLoss += profit.Profit
	} else if profit.Profit > 0 {
		s.AccumulatedProfit += profit.Profit
		s.TodayProfit += profit.Profit
	}
}

func (s *ProfitStats) AddTrade(trade types.Trade) {
	if s.IsOver24Hours() {
		s.ResetToday()
	}

	s.AccumulatedVolume += fixedpoint.NewFromFloat(trade.Quantity)
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

package bbgo

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

// Profit struct stores the PnL information
type Profit struct {
	Symbol string `json:"symbol"`

	// Profit is the profit of this trade made. negative profit means loss.
	Profit fixedpoint.Value `json:"profit" db:"profit"`

	// NetProfit is (profit - trading fee)
	NetProfit   fixedpoint.Value `json:"netProfit" db:"net_profit"`
	AverageCost fixedpoint.Value `json:"averageCost" db:"average_ost"`

	TradeAmount float64 `json:"tradeAmount" db:"trade_amount"`

	// ProfitMargin is a percentage of the profit and the capital amount
	ProfitMargin fixedpoint.Value `json:"profitMargin" db:"profit_margin"`

	// NetProfitMargin is a percentage of the net profit and the capital amount
	NetProfitMargin fixedpoint.Value `json:"netProfitMargin" db:"net_profit_margin"`

	QuoteCurrency string `json:"quote_currency" db:"quote_currency"`
	BaseCurrency  string `json:"base_currency" db:"base_currency"`

	// FeeInUSD is the summed fee of this profit,
	// you will need to convert the trade fee into USD since the fee currencies can be different.
	FeeInUSD           fixedpoint.Value `json:"feeInUSD" db:"fee_in_usd"`
	Time               time.Time        `json:"time" db:"time"`
	Strategy           string           `json:"strategy" db:"strategy"`
	StrategyInstanceID string           `json:"strategyInstanceID" db:"strategy_instance_id"`
}

func (p Profit) PlainText() string {
	return fmt.Sprintf("%s trade profit %s %f %s (%.2f%%), net profit =~ %f %s (%.2f%%)",
		p.Symbol,
		pnlEmoji(p.Profit),
		p.Profit.Float64(), p.QuoteCurrency,
		p.ProfitMargin.Float64()*100.0,
		p.NetProfit.Float64(), p.QuoteCurrency,
		p.NetProfitMargin.Float64()*100.0,
	)
}

var lossEmoji = "🔥"
var profitEmoji = "💰"

func pnlEmoji(pnl fixedpoint.Value) string {
	if pnl < 0 {
		return lossEmoji
	}

	if pnl == 0 {
		return ""
	}

	return profitEmoji
}

type ProfitStats struct {
	Symbol        string `json:"symbol"`
	QuoteCurrency string `json:"quoteCurrency"`
	BaseCurrency  string `json:"baseCurrency"`

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

func (s *ProfitStats) PlainText() string {
	since := time.Unix(s.AccumulatedSince, 0).Local()
	return fmt.Sprintf("today %s profit %f %s,\n"+
		"today %s net profit %f %s,\n"+
		"today %s trade loss %f %s\n"+
		"accumulated profit %f %s,\n"+
		"accumulated net profit %f %s,\n"+
		"accumulated trade loss %f %s\n"+
		"since %s",
		s.Symbol, s.TodayPnL.Float64(), s.QuoteCurrency,
		s.Symbol, s.TodayNetProfit.Float64(), s.QuoteCurrency,
		s.Symbol, s.TodayLoss.Float64(), s.QuoteCurrency,
		s.AccumulatedPnL.Float64(), s.QuoteCurrency,
		s.AccumulatedNetProfit.Float64(), s.QuoteCurrency,
		s.AccumulatedLoss.Float64(), s.QuoteCurrency,
		since.Format(time.RFC822),
	)
}

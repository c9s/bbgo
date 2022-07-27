package types

import (
	"time"

	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type IntervalProfitCollector struct {
	Interval Interval      `json:"interval"`
	Profits  *Float64Slice `json:"profits"`
	tmpTime  time.Time     `json:"tmpTime"`
}

func NewIntervalProfitCollector(i Interval, startTime time.Time) *IntervalProfitCollector {
	return &IntervalProfitCollector{Interval: i, tmpTime: startTime, Profits: &Float64Slice{1.}}
}

// Update the collector by every traded profit
func (s *IntervalProfitCollector) Update(profit *Profit) {
	if s.tmpTime.IsZero() {
		panic("No valid start time. Did you create IntervalProfitCollector instance using NewIntervalProfitCollector?")
	} else {
		duration := s.Interval.Duration()
		if profit.TradedAt.Before(s.tmpTime.Add(duration)) {
			(*s.Profits)[len(*s.Profits)-1] *= 1. + profit.NetProfitMargin.Float64()
		} else {
			for {
				s.Profits.Update(1.)
				s.tmpTime = s.tmpTime.Add(duration)
				if profit.TradedAt.Before(s.tmpTime.Add(duration)) {
					(*s.Profits)[len(*s.Profits)-1] *= 1. + profit.NetProfitMargin.Float64()
					break
				}
			}
		}
	}
}

// Get number of profitable traded intervals
func (s *IntervalProfitCollector) GetNumOfProfitableIntervals() (profit int) {
	if s.Profits == nil {
		panic("profits array empty. Did you create IntervalProfitCollector instance using NewIntervalProfitCollector?")
	}
	for _, v := range *s.Profits {
		if v > 1. {
			profit += 1
		}
	}
	return profit
}

// Get number of non-profitable traded intervals
// (no trade within the interval or pnl = 0 will be also included here)
func (s *IntervalProfitCollector) GetNumOfNonProfitableIntervals() (nonprofit int) {
	if s.Profits == nil {
		panic("profits array empty. Did you create IntervalProfitCollector instance using NewIntervalProfitCollector?")
	}
	for _, v := range *s.Profits {
		if v <= 1. {
			nonprofit += 1
		}
	}
	return nonprofit
}

// Get sharpe value with the interval of profit collected.
// no smart sharpe ON for the calculated result
func (s *IntervalProfitCollector) GetSharpe() float64 {
	if s.tmpTime.IsZero() {
		panic("No valid start time. Did you create IntervalProfitCollector instance using NewIntervalProfitCollector?")
	}
	if s.Profits == nil {
		panic("profits array empty. Did you create IntervalProfitCollector instance using NewIntervalProfitCollector?")
	}
	return Sharpe(Minus(s.Profits, 1.), s.Profits.Length(), true, false)
}

func (s *IntervalProfitCollector) GetOmega() float64 {
	return Omega(Minus(s.Profits, 1.))
}

func (s IntervalProfitCollector) MarshalYAML() (interface{}, error) {
	result := make(map[string]interface{})
	result["Sharpe Ratio"] = s.GetSharpe()
	result["Omega Ratio"] = s.GetOmega()
	result["Profitable Count"] = s.GetNumOfProfitableIntervals()
	result["NonProfitable Count"] = s.GetNumOfNonProfitableIntervals()
	return result, nil
}

// TODO: Add more stats from the reference:
// See https://www.metatrader5.com/en/terminal/help/algotrading/testing_report
type TradeStats struct {
	Symbol              string                                `json:"symbol"`
	WinningRatio        fixedpoint.Value                      `json:"winningRatio" yaml:"winningRatio"`
	NumOfLossTrade      int                                   `json:"numOfLossTrade" yaml:"numOfLossTrade"`
	NumOfProfitTrade    int                                   `json:"numOfProfitTrade" yaml:"numOfProfitTrade"`
	GrossProfit         fixedpoint.Value                      `json:"grossProfit" yaml:"grossProfit"`
	GrossLoss           fixedpoint.Value                      `json:"grossLoss" yaml:"grossLoss"`
	Profits             []fixedpoint.Value                    `json:"profits" yaml:"profits"`
	Losses              []fixedpoint.Value                    `json:"losses" yaml:"losses"`
	MostProfitableTrade fixedpoint.Value                      `json:"mostProfitableTrade" yaml:"mostProfitableTrade"`
	MostLossTrade       fixedpoint.Value                      `json:"mostLossTrade" yaml:"mostLossTrade"`
	ProfitFactor        fixedpoint.Value                      `json:"profitFactor" yaml:"profitFactor"`
	TotalNetProfit      fixedpoint.Value                      `json:"totalNetProfit" yaml:"totalNetProfit"`
	IntervalProfits     map[Interval]*IntervalProfitCollector `jons:"intervalProfits,omitempty" yaml: "intervalProfits,omitempty"`
}

func NewTradeStats(symbol string) *TradeStats {
	return &TradeStats{Symbol: symbol, IntervalProfits: make(map[Interval]*IntervalProfitCollector)}
}

// Set IntervalProfitCollector explicitly to enable the sharpe ratio calculation
func (s *TradeStats) SetIntervalProfitCollector(c *IntervalProfitCollector) {
	s.IntervalProfits[c.Interval] = c
}

func (s *TradeStats) Add(profit *Profit) {
	if profit.Symbol != s.Symbol {
		return
	}

	s.add(profit.Profit)
	for _, v := range s.IntervalProfits {
		v.Update(profit)
	}
}

func (s *TradeStats) add(pnl fixedpoint.Value) {
	if pnl.Sign() > 0 {
		s.NumOfProfitTrade++
		s.Profits = append(s.Profits, pnl)
		s.GrossProfit = s.GrossProfit.Add(pnl)
		s.MostProfitableTrade = fixedpoint.Max(s.MostProfitableTrade, pnl)
	} else {
		s.NumOfLossTrade++
		s.Losses = append(s.Losses, pnl)
		s.GrossLoss = s.GrossLoss.Add(pnl)
		s.MostLossTrade = fixedpoint.Min(s.MostLossTrade, pnl)
	}
	s.TotalNetProfit = s.TotalNetProfit.Add(pnl)

	// The win/loss ratio is your wins divided by your losses.
	// In the example, suppose for the sake of simplicity that 60 trades were winners, and 40 were losers.
	// Your win/loss ratio would be 60/40 = 1.5. That would mean that you are winning 50% more often than you are losing.
	if s.NumOfLossTrade == 0 && s.NumOfProfitTrade > 0 {
		s.WinningRatio = fixedpoint.One
	} else {
		s.WinningRatio = fixedpoint.NewFromFloat(float64(s.NumOfProfitTrade) / float64(s.NumOfLossTrade))
	}

	s.ProfitFactor = s.GrossProfit.Div(s.GrossLoss.Abs())
}

// Output TradeStats without Profits and Losses
func (s *TradeStats) BriefString() string {
	out, _ := yaml.Marshal(&TradeStats{
		Symbol:              s.Symbol,
		WinningRatio:        s.WinningRatio,
		NumOfLossTrade:      s.NumOfLossTrade,
		NumOfProfitTrade:    s.NumOfProfitTrade,
		GrossProfit:         s.GrossProfit,
		GrossLoss:           s.GrossLoss,
		MostProfitableTrade: s.MostProfitableTrade,
		MostLossTrade:       s.MostLossTrade,
		ProfitFactor:        s.ProfitFactor,
		TotalNetProfit:      s.TotalNetProfit,
		IntervalProfits:     s.IntervalProfits,
	})
	return string(out)
}

func (s *TradeStats) String() string {
	out, _ := yaml.Marshal(s)
	return string(out)
}

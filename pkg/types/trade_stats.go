package types

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type IntervalProfitCollector struct {
	Interval  Interval      `json:"interval"`
	Profits   *floats.Slice `json:"profits"`
	Timestamp *floats.Slice `json:"timestamp"`
	tmpTime   time.Time     `json:"tmpTime"`
}

func NewIntervalProfitCollector(i Interval, startTime time.Time) *IntervalProfitCollector {
	return &IntervalProfitCollector{Interval: i, tmpTime: startTime, Profits: &floats.Slice{1.}, Timestamp: &floats.Slice{float64(startTime.Unix())}}
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
				s.Timestamp.Update(float64(s.tmpTime.Unix()))
				if profit.TradedAt.Before(s.tmpTime.Add(duration)) {
					(*s.Profits)[len(*s.Profits)-1] *= 1. + profit.NetProfitMargin.Float64()
					break
				}
			}
		}
	}
}

type ProfitReport struct {
	StartTime time.Time `json:"startTime"`
	Profit    float64   `json:"profit"`
	Interval  Interval  `json:"interval"`
}

func (s ProfitReport) String() string {
	b, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	return string(b)
}

// Get all none-profitable intervals
func (s *IntervalProfitCollector) GetNonProfitableIntervals() (result []ProfitReport) {
	if s.Profits == nil {
		return result
	}
	l := s.Profits.Length()
	for i := 0; i < l; i++ {
		if s.Profits.Index(i) <= 1. {
			result = append(result, ProfitReport{StartTime: time.Unix(int64(s.Timestamp.Index(i)), 0), Profit: s.Profits.Index(i), Interval: s.Interval})
		}
	}
	return result
}

// Get all profitable intervals
func (s *IntervalProfitCollector) GetProfitableIntervals() (result []ProfitReport) {
	if s.Profits == nil {
		return result
	}
	l := s.Profits.Length()
	for i := 0; i < l; i++ {
		if s.Profits.Index(i) > 1. {
			result = append(result, ProfitReport{StartTime: time.Unix(int64(s.Timestamp.Index(i)), 0), Profit: s.Profits.Index(i), Interval: s.Interval})
		}
	}
	return result
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

// Get sortino value with the interval of profit collected.
// No risk-free return rate and smart sortino OFF for the calculated result.
func (s *IntervalProfitCollector) GetSortino() float64 {
	if s.tmpTime.IsZero() {
		panic("No valid start time. Did you create IntervalProfitCollector instance using NewIntervalProfitCollector?")
	}
	if s.Profits == nil {
		panic("profits array empty. Did you create IntervalProfitCollector instance using NewIntervalProfitCollector?")
	}
	return Sortino(Minus(s.Profits, 1.), 0., s.Profits.Length(), true, false)
}

func (s *IntervalProfitCollector) GetOmega() float64 {
	return Omega(Minus(s.Profits, 1.))
}

func (s IntervalProfitCollector) MarshalYAML() (interface{}, error) {
	result := make(map[string]interface{})
	result["Sharpe Ratio"] = s.GetSharpe()
	result["Sortino Ratio"] = s.GetSortino()
	result["Omega Ratio"] = s.GetOmega()
	result["Profitable Count"] = s.GetNumOfProfitableIntervals()
	result["NonProfitable Count"] = s.GetNumOfNonProfitableIntervals()
	return result, nil
}

// TODO: Add more stats from the reference:
// See https://www.metatrader5.com/en/terminal/help/algotrading/testing_report
type TradeStats struct {
	Symbol string `json:"symbol,omitempty"`

	WinningRatio     fixedpoint.Value `json:"winningRatio" yaml:"winningRatio"`
	NumOfLossTrade   int              `json:"numOfLossTrade" yaml:"numOfLossTrade"`
	NumOfProfitTrade int              `json:"numOfProfitTrade" yaml:"numOfProfitTrade"`

	GrossProfit fixedpoint.Value `json:"grossProfit" yaml:"grossProfit"`
	GrossLoss   fixedpoint.Value `json:"grossLoss" yaml:"grossLoss"`

	Profits []fixedpoint.Value `json:"profits,omitempty" yaml:"profits,omitempty"`
	Losses  []fixedpoint.Value `json:"losses,omitempty" yaml:"losses,omitempty"`

	orderProfits map[uint64][]*Profit

	LargestProfitTrade fixedpoint.Value `json:"largestProfitTrade,omitempty" yaml:"largestProfitTrade"`
	LargestLossTrade   fixedpoint.Value `json:"largestLossTrade,omitempty" yaml:"largestLossTrade"`
	AverageProfitTrade fixedpoint.Value `json:"averageProfitTrade" yaml:"averageProfitTrade"`
	AverageLossTrade   fixedpoint.Value `json:"averageLossTrade" yaml:"averageLossTrade"`

	ProfitFactor    fixedpoint.Value                      `json:"profitFactor" yaml:"profitFactor"`
	TotalNetProfit  fixedpoint.Value                      `json:"totalNetProfit" yaml:"totalNetProfit"`
	IntervalProfits map[Interval]*IntervalProfitCollector `json:"intervalProfits,omitempty" yaml:"intervalProfits,omitempty"`

	// MaximumConsecutiveWins - (counter) the longest series of winning trades
	MaximumConsecutiveWins int `json:"maximumConsecutiveWins" yaml:"maximumConsecutiveWins"`

	// MaximumConsecutiveLosses - (counter) the longest series of losing trades
	MaximumConsecutiveLosses int `json:"maximumConsecutiveLosses" yaml:"maximumConsecutiveLosses"`

	// MaximumConsecutiveProfit - ($) the longest series of winning trades and their total profit;
	MaximumConsecutiveProfit fixedpoint.Value `json:"maximumConsecutiveProfit" yaml:"maximumConsecutiveProfit"`

	// MaximumConsecutiveLoss - ($) the longest series of losing trades and their total loss;
	MaximumConsecutiveLoss fixedpoint.Value `json:"maximumConsecutiveLoss" yaml:"maximumConsecutiveLoss"`

	consecutiveSide    int
	consecutiveCounter int
	consecutiveAmount  fixedpoint.Value
}

func NewTradeStats(symbol string) *TradeStats {
	return &TradeStats{Symbol: symbol, IntervalProfits: make(map[Interval]*IntervalProfitCollector)}
}

// Set IntervalProfitCollector explicitly to enable the sharpe ratio calculation
func (s *TradeStats) SetIntervalProfitCollector(c *IntervalProfitCollector) {
	s.IntervalProfits[c.Interval] = c
}

func (s *TradeStats) CsvHeader() []string {
	return []string{
		"winningRatio",
		"numOfProfitTrade",
		"numOfLossTrade",
		"grossProfit",
		"grossLoss",
		"profitFactor",
		"largestProfitTrade",
		"largestLossTrade",
		"maximumConsecutiveWins",
		"maximumConsecutiveLosses",
	}
}

func (s *TradeStats) CsvRecords() [][]string {
	return [][]string{
		{
			s.WinningRatio.String(),
			strconv.Itoa(s.NumOfProfitTrade),
			strconv.Itoa(s.NumOfLossTrade),
			s.GrossProfit.String(),
			s.GrossLoss.String(),
			s.ProfitFactor.String(),
			s.LargestProfitTrade.String(),
			s.LargestLossTrade.String(),
			strconv.Itoa(s.MaximumConsecutiveWins),
			strconv.Itoa(s.MaximumConsecutiveLosses),
		},
	}
}

func (s *TradeStats) Add(profit *Profit) {
	if s.Symbol != "" && profit.Symbol != s.Symbol {
		return
	}

	if s.orderProfits == nil {
		s.orderProfits = make(map[uint64][]*Profit)
	}

	if profit.OrderID > 0 {
		s.orderProfits[profit.OrderID] = append(s.orderProfits[profit.OrderID], profit)
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
		s.LargestProfitTrade = fixedpoint.Max(s.LargestProfitTrade, pnl)

		// consecutive same side (made profit last time)
		if s.consecutiveSide == 0 || s.consecutiveSide == 1 {
			s.consecutiveSide = 1
			s.consecutiveCounter++
			s.consecutiveAmount = s.consecutiveAmount.Add(pnl)
		} else { // was loss, now profit, store the last loss and the loss amount
			s.MaximumConsecutiveLosses = int(math.Max(float64(s.MaximumConsecutiveLosses), float64(s.consecutiveCounter)))
			s.MaximumConsecutiveLoss = fixedpoint.Min(s.MaximumConsecutiveLoss, s.consecutiveAmount)

			s.consecutiveSide = 1
			s.consecutiveCounter = 0
			s.consecutiveAmount = pnl
		}

	} else {
		s.NumOfLossTrade++
		s.Losses = append(s.Losses, pnl)
		s.GrossLoss = s.GrossLoss.Add(pnl)
		s.LargestLossTrade = fixedpoint.Min(s.LargestLossTrade, pnl)

		// consecutive same side (made loss last time)
		if s.consecutiveSide == 0 || s.consecutiveSide == -1 {
			s.consecutiveSide = -1
			s.consecutiveCounter++
			s.consecutiveAmount = s.consecutiveAmount.Add(pnl)
		} else { // was profit, now loss, store the last win and profit
			s.MaximumConsecutiveWins = int(math.Max(float64(s.MaximumConsecutiveWins), float64(s.consecutiveCounter)))
			s.MaximumConsecutiveProfit = fixedpoint.Max(s.MaximumConsecutiveProfit, s.consecutiveAmount)

			s.consecutiveSide = -1
			s.consecutiveCounter = 0
			s.consecutiveAmount = pnl
		}

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
	if len(s.Profits) > 0 {
		s.AverageProfitTrade = fixedpoint.Avg(s.Profits)
	}
	if len(s.Losses) > 0 {
		s.AverageLossTrade = fixedpoint.Avg(s.Losses)
	}
}

// Output TradeStats without Profits and Losses
func (s *TradeStats) BriefString() string {
	out, _ := yaml.Marshal(&TradeStats{
		Symbol:                   s.Symbol,
		WinningRatio:             s.WinningRatio,
		NumOfLossTrade:           s.NumOfLossTrade,
		NumOfProfitTrade:         s.NumOfProfitTrade,
		GrossProfit:              s.GrossProfit,
		GrossLoss:                s.GrossLoss,
		LargestProfitTrade:       s.LargestProfitTrade,
		LargestLossTrade:         s.LargestLossTrade,
		AverageProfitTrade:       s.AverageProfitTrade,
		AverageLossTrade:         s.AverageLossTrade,
		ProfitFactor:             s.ProfitFactor,
		TotalNetProfit:           s.TotalNetProfit,
		IntervalProfits:          s.IntervalProfits,
		MaximumConsecutiveWins:   s.MaximumConsecutiveWins,
		MaximumConsecutiveLosses: s.MaximumConsecutiveLosses,
		MaximumConsecutiveProfit: s.MaximumConsecutiveProfit,
		MaximumConsecutiveLoss:   s.MaximumConsecutiveLoss,
	})
	return string(out)
}

func (s *TradeStats) String() string {
	out, _ := yaml.Marshal(s)
	return string(out)
}

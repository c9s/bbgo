package types

import (
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type TradeStats struct {
	WinningRatio        fixedpoint.Value   `json:"winningRatio" yaml:"winningRatio"`
	NumOfLossTrade      int                `json:"numOfLossTrade" yaml:"numOfLossTrade"`
	NumOfProfitTrade    int                `json:"numOfProfitTrade" yaml:"numOfProfitTrade"`
	GrossProfit         fixedpoint.Value   `json:"grossProfit" yaml:"grossProfit"`
	GrossLoss           fixedpoint.Value   `json:"grossLoss" yaml:"grossLoss"`
	Profits             []fixedpoint.Value `json:"profits" yaml:"profits"`
	Losses              []fixedpoint.Value `json:"losses" yaml:"losses"`
	MostProfitableTrade fixedpoint.Value   `json:"mostProfitableTrade" yaml:"mostProfitableTrade"`
	MostLossTrade       fixedpoint.Value   `json:"mostLossTrade" yaml:"mostLossTrade"`
	ProfitFactor        fixedpoint.Value   `json:"profitFactor" yaml:"profitFactor"`
	TotalNetProfit      fixedpoint.Value   `json:"totalNetProfit" yaml:"totalNetProfit"`
}

func (s *TradeStats) Add(pnl fixedpoint.Value) {
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

	s.ProfitFactor = s.GrossProfit.Div(s.GrossLoss)
}

func (s *TradeStats) String() string {
	out, _ := yaml.Marshal(s)
	return string(out)
}

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

	if s.NumOfLossTrade == 0 && s.NumOfProfitTrade > 0 {
		s.WinningRatio = fixedpoint.One
	} else {
		s.WinningRatio = fixedpoint.NewFromFloat(float64(s.NumOfProfitTrade) / float64(s.NumOfLossTrade))
	}
}

func (s *TradeStats) String() string {
	out, _ := yaml.Marshal(s)
	return string(out)
}

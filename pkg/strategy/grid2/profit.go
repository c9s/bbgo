package grid2

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type GridProfit struct {
	Currency string           `json:"currency"`
	Profit   fixedpoint.Value `json:"profit"`
	Time     time.Time        `json:"time"`
	Order    types.Order      `json:"order"`
}

type GridProfitStats struct {
	Symbol           string           `json:"symbol"`
	TotalBaseProfit  fixedpoint.Value `json:"totalBaseProfit"`
	TotalQuoteProfit fixedpoint.Value `json:"totalQuoteProfit"`
	FloatProfit      fixedpoint.Value `json:"floatProfit"`
	GridProfit       fixedpoint.Value `json:"gridProfit"`
	ArbitrageCount   int              `json:"arbitrageCount"`
	TotalFee         fixedpoint.Value `json:"totalFee"`
	Volume           fixedpoint.Value `json:"volume"`
	Market           types.Market     `json:"market"`
	ProfitEntries    []*GridProfit    `json:"profitEntries"`
}

func newGridProfitStats(market types.Market) *GridProfitStats {
	return &GridProfitStats{
		Symbol: market.Symbol,
		Market: market,
	}
}

func (s *GridProfitStats) AddProfit(profit *GridProfit) {
	switch profit.Currency {
	case s.Market.QuoteCurrency:
		s.TotalQuoteProfit = s.TotalQuoteProfit.Add(profit.Profit)
	case s.Market.BaseCurrency:
		s.TotalBaseProfit = s.TotalBaseProfit.Add(profit.Profit)
	}

	s.ProfitEntries = append(s.ProfitEntries, profit)
}

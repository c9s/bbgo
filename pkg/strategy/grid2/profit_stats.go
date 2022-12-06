package grid2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type GridProfitStats struct {
	Symbol           string                      `json:"symbol"`
	TotalBaseProfit  fixedpoint.Value            `json:"totalBaseProfit,omitempty"`
	TotalQuoteProfit fixedpoint.Value            `json:"totalQuoteProfit,omitempty"`
	FloatProfit      fixedpoint.Value            `json:"floatProfit,omitempty"`
	GridProfit       fixedpoint.Value            `json:"gridProfit,omitempty"`
	ArbitrageCount   int                         `json:"arbitrageCount,omitempty"`
	TotalFee         map[string]fixedpoint.Value `json:"totalFee,omitempty"`
	Volume           fixedpoint.Value            `json:"volume,omitempty"`
	Market           types.Market                `json:"market,omitempty"`
	ProfitEntries    []*GridProfit               `json:"profitEntries,omitempty"`
}

func newGridProfitStats(market types.Market) *GridProfitStats {
	return &GridProfitStats{
		Symbol:           market.Symbol,
		TotalBaseProfit:  fixedpoint.Zero,
		TotalQuoteProfit: fixedpoint.Zero,
		FloatProfit:      fixedpoint.Zero,
		GridProfit:       fixedpoint.Zero,
		ArbitrageCount:   0,
		TotalFee:         make(map[string]fixedpoint.Value),
		Volume:           fixedpoint.Zero,
		Market:           market,
		ProfitEntries:    nil,
	}
}

func (s *GridProfitStats) AddTrade(trade types.Trade) {
	if s.TotalFee == nil {
		s.TotalFee = make(map[string]fixedpoint.Value)
	}

	if fee, ok := s.TotalFee[trade.FeeCurrency]; ok {
		s.TotalFee[trade.FeeCurrency] = fee.Add(trade.Fee)
	} else {
		s.TotalFee[trade.FeeCurrency] = trade.Fee
	}
}

func (s *GridProfitStats) AddProfit(profit *GridProfit) {
	// increase arbitrage count per profit round
	s.ArbitrageCount++

	switch profit.Currency {
	case s.Market.QuoteCurrency:
		s.TotalQuoteProfit = s.TotalQuoteProfit.Add(profit.Profit)
	case s.Market.BaseCurrency:
		s.TotalBaseProfit = s.TotalBaseProfit.Add(profit.Profit)
	}

	s.ProfitEntries = append(s.ProfitEntries, profit)
}

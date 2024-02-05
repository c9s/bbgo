package dca2

import (
	"fmt"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitStats struct {
	Symbol string       `json:"symbol"`
	Market types.Market `json:"market,omitempty"`

	FromOrderID     uint64           `json:"fromOrderID,omitempty"`
	Round           int64            `json:"round,omitempty"`
	QuoteInvestment fixedpoint.Value `json:"quoteInvestment,omitempty"`

	CurrentRoundProfit fixedpoint.Value            `json:"currentRoundProfit,omitempty"`
	CurrentRoundFee    map[string]fixedpoint.Value `json:"currentRoundFee,omitempty"`
	TotalProfit        fixedpoint.Value            `json:"totalProfit,omitempty"`
	TotalFee           map[string]fixedpoint.Value `json:"totalFee,omitempty"`

	types.PersistenceTTL
}

func newProfitStats(market types.Market, quoteInvestment fixedpoint.Value) *ProfitStats {
	return &ProfitStats{
		Symbol:          market.Symbol,
		Market:          market,
		Round:           1,
		QuoteInvestment: quoteInvestment,
		CurrentRoundFee: make(map[string]fixedpoint.Value),
		TotalFee:        make(map[string]fixedpoint.Value),
	}
}

func (s *ProfitStats) AddTrade(trade types.Trade) {
	if s.CurrentRoundFee == nil {
		s.CurrentRoundFee = make(map[string]fixedpoint.Value)
	}

	if fee, ok := s.CurrentRoundFee[trade.FeeCurrency]; ok {
		s.CurrentRoundFee[trade.FeeCurrency] = fee.Add(trade.Fee)
	} else {
		s.CurrentRoundFee[trade.FeeCurrency] = trade.Fee
	}

	if s.TotalFee == nil {
		s.TotalFee = make(map[string]fixedpoint.Value)
	}

	if fee, ok := s.TotalFee[trade.FeeCurrency]; ok {
		s.TotalFee[trade.FeeCurrency] = fee.Add(trade.Fee)
	} else {
		s.TotalFee[trade.FeeCurrency] = trade.Fee
	}

	quoteQuantity := trade.QuoteQuantity
	if trade.Side == types.SideTypeBuy {
		quoteQuantity = quoteQuantity.Neg()
	}

	s.CurrentRoundProfit = s.CurrentRoundProfit.Add(quoteQuantity)
	s.TotalProfit = s.TotalProfit.Add(quoteQuantity)

	if s.Market.QuoteCurrency == trade.FeeCurrency {
		s.CurrentRoundProfit = s.CurrentRoundProfit.Sub(trade.Fee)
		s.TotalProfit = s.TotalProfit.Sub(trade.Fee)
	}
}

func (s *ProfitStats) NewRound() {
	s.Round++
	s.CurrentRoundProfit = fixedpoint.Zero
	s.CurrentRoundFee = make(map[string]fixedpoint.Value)
}

func (s *ProfitStats) String() string {
	var sb strings.Builder
	sb.WriteString("[------------------ Profit Stats ------------------]\n")
	sb.WriteString(fmt.Sprintf("Round: %d\n", s.Round))
	sb.WriteString(fmt.Sprintf("From Order ID: %d\n", s.FromOrderID))
	sb.WriteString(fmt.Sprintf("Quote Investment: %s\n", s.QuoteInvestment))
	sb.WriteString(fmt.Sprintf("Current Round Profit: %s\n", s.CurrentRoundProfit))
	sb.WriteString(fmt.Sprintf("Total Profit: %s\n", s.TotalProfit))
	for currency, fee := range s.CurrentRoundFee {
		sb.WriteString(fmt.Sprintf("FEE (%s): %s\n", currency, fee))
	}
	sb.WriteString("[------------------ Profit Stats ------------------]\n")

	return sb.String()
}

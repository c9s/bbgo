package dca2

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitStats struct {
	Symbol string       `json:"symbol"`
	Market types.Market `json:"market,omitempty"`

	CreatedAt       time.Time        `json:"since,omitempty"`
	UpdatedAt       time.Time        `json:"updatedAt,omitempty"`
	Round           int64            `json:"round,omitempty"`
	QuoteInvestment fixedpoint.Value `json:"quoteInvestment,omitempty"`

	RoundProfit fixedpoint.Value            `json:"roundProfit,omitempty"`
	RoundFee    map[string]fixedpoint.Value `json:"roundFee,omitempty"`
	TotalProfit fixedpoint.Value            `json:"totalProfit,omitempty"`
	TotalFee    map[string]fixedpoint.Value `json:"totalFee,omitempty"`

	// ttl is the ttl to keep in persistence
	ttl time.Duration
}

func newProfitStats(market types.Market, quoteInvestment fixedpoint.Value) *ProfitStats {
	return &ProfitStats{
		Symbol:          market.Symbol,
		Market:          market,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Round:           0,
		QuoteInvestment: quoteInvestment,
		RoundFee:        make(map[string]fixedpoint.Value),
		TotalFee:        make(map[string]fixedpoint.Value),
	}
}

func (s *ProfitStats) SetTTL(ttl time.Duration) {
	if ttl.Nanoseconds() <= 0 {
		return
	}
	s.ttl = ttl
}

func (s *ProfitStats) Expiration() time.Duration {
	return s.ttl
}

func (s *ProfitStats) AddTrade(trade types.Trade) {
	if s.RoundFee == nil {
		s.RoundFee = make(map[string]fixedpoint.Value)
	}

	if fee, ok := s.RoundFee[trade.FeeCurrency]; ok {
		s.RoundFee[trade.FeeCurrency] = fee.Add(trade.Fee)
	} else {
		s.RoundFee[trade.FeeCurrency] = trade.Fee
	}

	if s.TotalFee == nil {
		s.TotalFee = make(map[string]fixedpoint.Value)
	}

	if fee, ok := s.TotalFee[trade.FeeCurrency]; ok {
		s.TotalFee[trade.FeeCurrency] = fee.Add(trade.Fee)
	} else {
		s.TotalFee[trade.FeeCurrency] = trade.Fee
	}

	switch trade.Side {
	case types.SideTypeSell:
		s.RoundProfit = s.RoundProfit.Add(trade.QuoteQuantity)
		s.TotalProfit = s.TotalProfit.Add(trade.QuoteQuantity)
	case types.SideTypeBuy:
		s.RoundProfit = s.RoundProfit.Sub(trade.QuoteQuantity)
		s.TotalProfit = s.TotalProfit.Sub(trade.QuoteQuantity)
	default:
	}

	s.UpdatedAt = trade.Time.Time()
}

func (s *ProfitStats) FinishRound() {
	s.Round++
	s.RoundProfit = fixedpoint.Zero
	s.RoundFee = make(map[string]fixedpoint.Value)
}

package dca2

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitStats struct {
	Symbol string       `json:"symbol"`
	Market types.Market `json:"market,omitempty"`

	FromOrderID     uint64           `json:"fromOrderID,omitempty"`
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

	quoteQuantity := trade.QuoteQuantity
	if trade.Side == types.SideTypeBuy {
		quoteQuantity = quoteQuantity.Neg()
	}

	s.RoundProfit = s.RoundProfit.Add(quoteQuantity)
	s.TotalProfit = s.TotalProfit.Add(quoteQuantity)

	if s.Market.QuoteCurrency == trade.FeeCurrency {
		s.RoundProfit.Sub(trade.Fee)
		s.TotalProfit.Sub(trade.Fee)
	}
}

func (s *ProfitStats) NewRound() {
	s.Round++
	s.RoundProfit = fixedpoint.Zero
	s.RoundFee = make(map[string]fixedpoint.Value)
}

func (s *ProfitStats) CalculateProfitOfRound(ctx context.Context, exchange types.Exchange) error {
	historyService, ok := exchange.(types.ExchangeTradeHistoryService)
	if !ok {
		return fmt.Errorf("exchange %s doesn't support ExchangeTradeHistoryService", exchange.Name())
	}

	queryService, ok := exchange.(types.ExchangeOrderQueryService)
	if !ok {
		return fmt.Errorf("exchange %s doesn't support ExchangeOrderQueryService", exchange.Name())
	}

	// query the orders of this round
	orders, err := historyService.QueryClosedOrders(ctx, s.Symbol, time.Time{}, time.Time{}, s.FromOrderID)
	if err != nil {
		return err
	}

	// query the trades of this round
	for _, order := range orders {
		if order.ExecutedQuantity.Sign() == 0 {
			// skip no trade orders
			continue
		}

		trades, err := queryService.QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  order.Symbol,
			OrderID: strconv.FormatUint(order.OrderID, 10),
		})

		if err != nil {
			return err
		}

		for _, trade := range trades {
			s.AddTrade(trade)
		}
	}

	s.FromOrderID = s.FromOrderID + 1
	s.QuoteInvestment = s.QuoteInvestment.Add(s.RoundProfit)

	return nil
}

func (s *ProfitStats) String() string {
	var sb strings.Builder
	sb.WriteString("[------------------ Profit Stats ------------------]\n")
	sb.WriteString(fmt.Sprintf("Round: %d\n", s.Round))
	sb.WriteString(fmt.Sprintf("From Order ID: %d\n", s.FromOrderID))
	sb.WriteString(fmt.Sprintf("Quote Investment: %s\n", s.QuoteInvestment))
	sb.WriteString(fmt.Sprintf("Round Profit: %s\n", s.RoundProfit))
	sb.WriteString(fmt.Sprintf("Total Profit: %s\n", s.TotalProfit))
	for currency, fee := range s.RoundFee {
		sb.WriteString(fmt.Sprintf("FEE (%s): %s\n", currency, fee))
	}
	sb.WriteString("[------------------ Profit Stats ------------------]\n")

	return sb.String()
}

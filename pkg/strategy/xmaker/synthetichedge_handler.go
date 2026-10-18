package xmaker

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/tradeid"
	"github.com/c9s/bbgo/pkg/types"
)

// when receiving trades from the source session,
// mock a trade with the quote amount and add to the fiat position
//
// hedge flow:
//
//	maker trade -> update source market position delta
//	source market position delta -> convert to fiat trade -> update fiat market position delta
func (s *SyntheticHedge) handleSourceTrade(trade types.Trade) {
	fiatDelta, ok := s.convertSourceTrade(trade)
	if !ok {
		return
	}

	// add position delta to let the fiat market hedge the position
	// note: do not hold the lock here, the fiat market worker could be waiting for the lock in its trade callback
	s.fiatMarket.positionDeltaC <- fiatDelta
}

// convertSourceTrade converts the source trade into the fiat trade, and returns the fiat position delta
func (s *SyntheticHedge) convertSourceTrade(trade types.Trade) (fixedpoint.Value, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isSplitSource() {
		// aggregate the trades from all the source markets into one source position,
		// so that we can use the average cost to convert the fiat trades back.
		trade.Symbol = s.sourcePosition.Symbol
		s.sourcePosition.AddTrade(trade)
	}

	cost := fixedpoint.Zero

	// 1) get the fiat book price from the snapshot when possible
	bid, ask := s.fiatMarket.GetQuotePrice()

	// 2) build up fiat position from the trade quote quantity
	fiatQuantity := trade.QuoteQuantity
	fiatDelta := fiatQuantity

	// reverse side because we assume source's quote currency is the fiat market's base currency here
	fiatOrderSide := trade.Side

	if s.forward {
		fiatQuantity = trade.QuoteQuantity
		fiatOrderSide = trade.Side
	} else {
		fiatQuantity = fixedpoint.One.Div(trade.QuoteQuantity)
		fiatOrderSide = trade.Side.Reverse()
	}

	fiatBaseSide := fiatOrderSide.Reverse()

	switch fiatOrderSide {
	case types.SideTypeBuy:
		cost = ask
		fiatDelta = fiatQuantity.Neg()
	case types.SideTypeSell:
		cost = bid
		fiatDelta = fiatQuantity
	default:
		return fixedpoint.Zero, false
	}

	// 3) convert source trade into fiat trade as the average cost of the fiat market position
	// This assumes source's quote currency is the fiat market's base currency
	fiatTrade := s.fiatMarket.newMockTrade(fiatBaseSide, cost, fiatQuantity, trade.Time.Time())
	if profit, netProfit, madeProfit := s.fiatMarket.Position.AddTrade(fiatTrade); madeProfit {
		// TODO: record the profits in somewhere?
		_ = profit
		_ = netProfit
	}

	return fiatDelta, true
}

func (s *SyntheticHedge) handleFiatTrade(strategy *Strategy, trade types.Trade) {
	avgCost, baseQuantity := s.closeSourcePosition(trade)

	// close the maker position
	// create a mock trade for closing the maker position
	// This assumes the source market's quote currency is the fiat market's base currency
	price := fixedpoint.Zero
	if s.forward {
		price = avgCost.Mul(trade.Price)
	} else {
		price = avgCost.Div(trade.Price)
	}

	side := trade.Side
	tradeId := tradeid.GlobalGenerator.Generate()

	syntheticTrade := types.Trade{
		ID:            tradeId,
		OrderID:       tradeId,
		Exchange:      strategy.makerSession.ExchangeName,
		Symbol:        strategy.makerMarket.Symbol,
		Price:         price,
		Quantity:      baseQuantity,
		QuoteQuantity: baseQuantity.Mul(price),
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       false,
		Time:          trade.Time,
		Fee:           trade.Fee,
		FeeCurrency:   trade.FeeCurrency,
		Tag:           TradeTagMock,
	}
	s.logger.Infof("[syntheticHedge] synthetic trade created: %+v, average cost: %f", syntheticTrade, avgCost.Float64())
	syntheticOrder := newMockOrderFromTrade(syntheticTrade, types.OrderTypeMarket)
	strategy.orderStore.Add(syntheticOrder)
	strategy.tradeCollector.TradeStore().Add(syntheticTrade)
	strategy.tradeCollector.Process()
}

// closeSourcePosition converts the fiat trade into the source trade to close the source position,
// it returns the source average cost and the closed base quantity
func (s *SyntheticHedge) closeSourcePosition(trade types.Trade) (avgCost, baseQuantity fixedpoint.Value) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourcePosition := s.getSourcePosition()
	avgCost = sourcePosition.AverageCost

	// convert the trade quantity to the source market's base currency quantity
	// calculate how much base quantity we can close the source position
	if s.forward {
		baseQuantity = trade.Quantity.Div(avgCost)
	} else {
		// To handle the case where the source market's quote currency is not the fiat market's base currency:
		// Assume it's TWD/USDT, the makerExchange rate is 0.03125 when USDT/TWD is at 32.0,
		// and trade quantity is in TWD,
		// so we need to convert Quantity from TWD to USDT unit by div
		baseQuantity = avgCost.Div(trade.Quantity.Div(trade.Price))
	}

	// convert the fiat trade into source trade to close the source position
	// side is reversed because we are closing the source hedge position
	mockSourceTrade := s.newMockSourceTrade(trade.Side.Reverse(), avgCost, baseQuantity, trade.Time.Time())
	if profit, netProfit, madeProfit := sourcePosition.AddTrade(mockSourceTrade); madeProfit {
		_ = profit
		_ = netProfit
	}

	return avgCost, baseQuantity
}

func (s *SyntheticHedge) newMockSourceTrade(
	side types.SideType, price, quantity fixedpoint.Value, tradeTime time.Time,
) types.Trade {
	if !s.isSplitSource() {
		return s.sourceMarket.newMockTrade(side, price, quantity, tradeTime)
	}

	trade := s.sourceMarkets()[0].newMockTrade(side, price, quantity, tradeTime)
	trade.Symbol = s.sourcePosition.Symbol
	return trade
}

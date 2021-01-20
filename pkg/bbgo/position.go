package bbgo

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Position struct {
	Symbol        string `json:"symbol"`
	BaseCurrency  string `json:"baseCurrency"`
	QuoteCurrency string `json:"quoteCurrency"`

	Base        fixedpoint.Value `json:"base"`
	Quote       fixedpoint.Value `json:"quote"`
	AverageCost fixedpoint.Value `json:"averageCost"`
}

func (p *Position) BindStream(stream types.Stream) {
	stream.OnTradeUpdate(func(trade types.Trade) {
		if p.Symbol == trade.Symbol {
			p.AddTrade(trade)
		}
	})
}

func (p *Position) AddTrades(trades []types.Trade) (fixedpoint.Value, bool) {
	var totalProfitAmount fixedpoint.Value
	for _, trade := range trades {
		if profitAmount, profit := p.AddTrade(trade); profit {
			totalProfitAmount += profitAmount
		}
	}

	return totalProfitAmount, totalProfitAmount != 0
}

func (p *Position) AddTrade(t types.Trade) (fixedpoint.Value, bool) {
	price := fixedpoint.NewFromFloat(t.Price)
	quantity := fixedpoint.NewFromFloat(t.Quantity)

	// Base > 0 means we're in long position
	// Base < 0  means we're in short position
	switch t.Side {

	case types.SideTypeBuy:
		if p.Base < 0 {
			// handling short-to-long position
			if p.Base+quantity > 0 {
				closingProfit := (p.AverageCost - price).Mul(-p.Base)

				p.Base += quantity
				p.Quote -= quantity.Mul(price)
				p.AverageCost = price

				return closingProfit, true
			} else {
				// covering short position
				p.Base += quantity
				p.Quote -= fixedpoint.NewFromFloat(t.QuoteQuantity)
				return (p.AverageCost - price).Mul(quantity), true
			}
		}

		if p.AverageCost == 0 {
			p.AverageCost = price
		} else {
			p.AverageCost = (p.AverageCost.Mul(p.Base) + price.Mul(quantity)).Div(p.Base + quantity)
		}

		p.Base += quantity
		p.Quote -= fixedpoint.NewFromFloat(t.QuoteQuantity)
		return 0, false

	case types.SideTypeSell:
		if p.Base > 0 {
			// long-to-short
			if p.Base-quantity < 0 {
				closingProfit := (price - p.AverageCost).Mul(p.Base)
				p.Base -= quantity
				p.Quote += quantity.Mul(price)
				p.AverageCost = price
				return closingProfit, true
			} else {
				p.Base -= quantity
				p.Quote += fixedpoint.NewFromFloat(t.QuoteQuantity)
				return (price - p.AverageCost).Mul(quantity), true
			}
		}

		// handling short position
		if p.AverageCost == 0 {
			p.AverageCost = price
		} else {
			p.AverageCost = (p.AverageCost.Mul(-p.Base) + price.Mul(quantity)).Div(-p.Base + quantity)
		}

		p.Base -= quantity
		p.Quote += fixedpoint.NewFromFloat(t.QuoteQuantity)

		return 0, false
	}

	return 0, false
}

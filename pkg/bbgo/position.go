package bbgo

import (
	"fmt"
	"sync"

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

	sync.Mutex
}

func (p *Position) String() string {
	return fmt.Sprintf("%s: average cost = %f, base = %f, quote = %f",
		p.Symbol,
		p.AverageCost.Float64(),
		p.Base.Float64(),
		p.Quote.Float64(),
	)
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
	p.Lock()
	defer p.Unlock()

	price := fixedpoint.NewFromFloat(t.Price)
	quantity := fixedpoint.NewFromFloat(t.Quantity)
	quoteQuantity := fixedpoint.NewFromFloat(t.QuoteQuantity)
	fee := fixedpoint.NewFromFloat(t.Fee)

	switch t.FeeCurrency {

	case p.BaseCurrency:
		quantity -= fee

	case p.QuoteCurrency:
		quoteQuantity -= fee

	}

	// Base > 0 means we're in long position
	// Base < 0  means we're in short position
	switch t.Side {

	case types.SideTypeBuy:
		if p.Base < 0 {
			// handling short-to-long position
			if p.Base+quantity > 0 {
				closingProfit := (p.AverageCost - price).Mul(-p.Base)
				p.Base += quantity
				p.Quote -= quoteQuantity
				p.AverageCost = price
				return closingProfit, true
			} else {
				// covering short position
				p.Base += quantity
				p.Quote -= quoteQuantity
				return (p.AverageCost - price).Mul(quantity), true
			}
		}

		p.AverageCost = (p.AverageCost.Mul(p.Base) + quoteQuantity).Div(p.Base + quantity)
		p.Base += quantity
		p.Quote -= quoteQuantity

		return 0, false

	case types.SideTypeSell:
		if p.Base > 0 {
			// long-to-short
			if p.Base-quantity < 0 {
				closingProfit := (price - p.AverageCost).Mul(p.Base)
				p.Base -= quantity
				p.Quote += quoteQuantity
				p.AverageCost = price
				return closingProfit, true
			} else {
				p.Base -= quantity
				p.Quote += quoteQuantity
				return (price - p.AverageCost).Mul(quantity), true
			}
		}

		// handling short position
		p.AverageCost = (p.AverageCost.Mul(-p.Base) + quoteQuantity).Div(-p.Base + quantity)
		p.Base -= quantity
		p.Quote += quoteQuantity

		return 0, false
	}

	return 0, false
}

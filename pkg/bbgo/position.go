package bbgo

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Position struct {
	Base        fixedpoint.Value
	Quote       fixedpoint.Value
	AverageCost fixedpoint.Value
}

func (p *Position) AddTrade(t types.Trade) (fixedpoint.Value, bool) {
	price := fixedpoint.NewFromFloat(t.Price)
	quantity := fixedpoint.NewFromFloat(t.Quantity)

	switch t.Side {

	case types.SideTypeBuy:

		if p.AverageCost == 0 {
			p.AverageCost = price
		} else {
			p.AverageCost = (p.AverageCost.Mul(p.Base) + price.Mul(quantity)).Div(p.Base + quantity)
		}

		p.Base += quantity
		p.Quote -= fixedpoint.NewFromFloat(t.QuoteQuantity)

		return 0, false

	case types.SideTypeSell:
		p.Base -= quantity
		p.Quote += fixedpoint.NewFromFloat(t.QuoteQuantity)

		return price - p.AverageCost, true
	}

	return 0, false
}

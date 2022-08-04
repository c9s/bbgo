package trinity

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type MultiCurrencyPosition struct {
	Currencies map[string]fixedpoint.Value
	Markets    map[string]types.Market
}

func (p *MultiCurrencyPosition) handleTrade(trade types.Trade) {
	market := p.Markets[trade.Symbol]
	switch trade.Side {
	case types.SideTypeBuy:
		p.Currencies[market.BaseCurrency] = p.Currencies[market.BaseCurrency].Add(trade.Quantity)
		p.Currencies[market.QuoteCurrency] = p.Currencies[market.QuoteCurrency].Sub(trade.QuoteQuantity)

	case types.SideTypeSell:
		p.Currencies[market.BaseCurrency] = p.Currencies[market.BaseCurrency].Sub(trade.Quantity)
		p.Currencies[market.QuoteCurrency] = p.Currencies[market.QuoteCurrency].Add(trade.QuoteQuantity)
	}
}

func (p *MultiCurrencyPosition) String() (o string) {
	for currency, base := range p.Currencies {
		if base.IsZero() {
			continue
		}

		o += fmt.Sprintf("base %s: %f", currency, base.Float64())
	}

	return o
}

func (p *MultiCurrencyPosition) BindStream(stream types.Stream) {
	stream.OnTradeUpdate(p.handleTrade)
}

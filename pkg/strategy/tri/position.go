package tri

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/currency"
)

type MultiCurrencyPosition struct {
	Currencies   map[string]fixedpoint.Value `json:"currencies"`
	Markets      map[string]types.Market     `json:"markets"`
	TotalProfits map[string]fixedpoint.Value `json:"totalProfits"`
	Fees         map[string]fixedpoint.Value `json:"fees"`
	TradePrices  map[string]fixedpoint.Value `json:"prices"`
}

func NewMultiCurrencyPosition(markets map[string]types.Market) *MultiCurrencyPosition {
	p := &MultiCurrencyPosition{
		Currencies:   make(map[string]fixedpoint.Value),
		Markets:      make(map[string]types.Market),
		TotalProfits: make(map[string]fixedpoint.Value),
		TradePrices:  make(map[string]fixedpoint.Value),
		Fees:         make(map[string]fixedpoint.Value),
	}

	for _, market := range markets {
		p.Markets[market.Symbol] = market
		p.Currencies[market.BaseCurrency] = fixedpoint.Zero
		p.Currencies[market.QuoteCurrency] = fixedpoint.Zero
		p.TotalProfits[market.QuoteCurrency] = fixedpoint.Zero
		p.TotalProfits[market.BaseCurrency] = fixedpoint.Zero
		p.Fees[market.QuoteCurrency] = fixedpoint.Zero
		p.Fees[market.BaseCurrency] = fixedpoint.Zero
		p.TradePrices[market.QuoteCurrency] = fixedpoint.Zero
		p.TradePrices[market.BaseCurrency] = fixedpoint.Zero
	}

	return p
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

	if currency.IsUSDFiatCurrency(market.QuoteCurrency) {
		p.TradePrices[market.BaseCurrency] = trade.Price
	} else if currency.IsUSDFiatCurrency(market.BaseCurrency) { // For USDT/TWD pair, convert USDT/TWD price to TWD/USDT
		p.TradePrices[market.QuoteCurrency] = one.Div(trade.Price)
	}

	if !trade.Fee.IsZero() {
		if f, ok := p.Fees[trade.FeeCurrency]; ok {
			p.Fees[trade.FeeCurrency] = f.Add(trade.Fee)
		} else {
			p.Fees[trade.FeeCurrency] = trade.Fee
		}
	}
}

func (p *MultiCurrencyPosition) CollectProfits() []Profit {
	var profits []Profit
	for cu, base := range p.Currencies {
		if base.IsZero() {
			continue
		}

		profit := Profit{
			Asset:       cu,
			Profit:      base,
			ProfitInUSD: fixedpoint.Zero,
		}

		if price, ok := p.TradePrices[cu]; ok && !price.IsZero() {
			profit.ProfitInUSD = base.Mul(price)
		} else if currency.IsUSDFiatCurrency(cu) {
			profit.ProfitInUSD = base
		}

		profits = append(profits, profit)

		if total, ok := p.TotalProfits[cu]; ok {
			p.TotalProfits[cu] = total.Add(base)
		} else {
			p.TotalProfits[cu] = base
		}
	}

	p.Reset()
	return profits
}

func (p *MultiCurrencyPosition) Reset() {
	for cu := range p.Currencies {
		p.Currencies[cu] = fixedpoint.Zero
	}
}

func (p *MultiCurrencyPosition) String() (o string) {
	o += "position: \n"

	for cu, base := range p.Currencies {
		if base.IsZero() {
			continue
		}

		o += fmt.Sprintf("- %s: %f\n", cu, base.Float64())
	}

	o += "totalProfits: \n"

	for cu, total := range p.TotalProfits {
		if total.IsZero() {
			continue
		}

		o += fmt.Sprintf("- %s: %f\n", cu, total.Float64())
	}

	o += "fees: \n"

	for cu, fee := range p.Fees {
		if fee.IsZero() {
			continue
		}

		o += fmt.Sprintf("- %s: %f\n", cu, fee.Float64())
	}

	return o
}

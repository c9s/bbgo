package tri

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/sigchan"
	"github.com/c9s/bbgo/pkg/types"
)

type ArbMarket struct {
	Symbol                      string
	BaseCurrency, QuoteCurrency string
	market                      types.Market

	stream            types.Stream
	book              *types.StreamOrderBook
	bestBid, bestAsk  types.PriceVolume
	buyRate, sellRate float64
	sigC              sigchan.Chan
}

func (m *ArbMarket) String() string {
	return m.Symbol
}

func (m *ArbMarket) getInitialBalance(balances types.BalanceMap, dir int) (fixedpoint.Value, string) {
	if dir == 1 { // sell 1 BTC -> 19000 USDT
		b, ok := balances[m.BaseCurrency]
		if !ok {
			return fixedpoint.Zero, m.BaseCurrency
		}

		return m.market.TruncateQuantity(b.Available), m.BaseCurrency
	} else if dir == -1 {
		b, ok := balances[m.QuoteCurrency]
		if !ok {
			return fixedpoint.Zero, m.QuoteCurrency
		}

		return m.market.TruncateQuantity(b.Available), m.QuoteCurrency
	}

	return fixedpoint.Zero, ""
}

func (m *ArbMarket) calculateRatio(dir int) float64 {
	if dir == 1 { // direct 1 = sell
		if m.bestBid.Price.IsZero() || m.bestBid.Volume.Compare(m.market.MinQuantity) <= 0 {
			return 0.0
		}

		return m.sellRate
	} else if dir == -1 {
		if m.bestAsk.Price.IsZero() || m.bestAsk.Volume.Compare(m.market.MinQuantity) <= 0 {
			return 0.0
		}

		return m.buyRate
	}

	return 0.0
}

func (m *ArbMarket) updateRate() {
	m.buyRate = 1.0 / m.bestAsk.Price.Float64()
	m.sellRate = m.bestBid.Price.Float64()

	if m.bestBid.Volume.Compare(m.market.MinQuantity) <= 0 && m.bestAsk.Volume.Compare(m.market.MinQuantity) <= 0 {
		return
	}

	m.sigC.Emit()
}

func (m *ArbMarket) newOrder(dir int, transitingQuantity float64) (types.SubmitOrder, float64) {
	if dir == 1 { // sell ETH -> BTC, sell USDT -> TWD
		q, r := fitQuantityByBase(m.market.TruncateQuantity(m.bestBid.Volume).Float64(), transitingQuantity)
		fq := fixedpoint.NewFromFloat(q)
		return types.SubmitOrder{
			Symbol:   m.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimit,
			Quantity: fq,
			Price:    m.bestBid.Price,
			Market:   m.market,
		}, r
	} else if dir == -1 { // use 1 BTC to buy X ETH
		q, r := fitQuantityByQuote(m.bestAsk.Price.Float64(), m.market.TruncateQuantity(m.bestAsk.Volume).Float64(), transitingQuantity)
		fq := fixedpoint.NewFromFloat(q)
		return types.SubmitOrder{
			Symbol:   m.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Quantity: fq,
			Price:    m.bestAsk.Price,
			Market:   m.market,
		}, r
	} else {
		panic(fmt.Errorf("unexpected direction: %v, valid values are (1, -1)", dir))
	}

	return types.SubmitOrder{}, 0.0
}

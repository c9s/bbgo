package tri

import (
	"fmt"
	"math"
	"strconv"

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

	truncateBaseQuantity, truncateQuoteQuantity QuantityTruncator
}

func (m *ArbMarket) String() string {
	return m.Symbol
}

type QuantityTruncator func(value fixedpoint.Value) fixedpoint.Value

func createBaseQuantityTruncator(m types.Market) QuantityTruncator {
	var stepSizeFloat = m.StepSize.Float64()
	var truncPrec = int(math.Round(math.Log10(stepSizeFloat) * -1.0))
	return createRoundedTruncator(truncPrec)
}

func createPricePrecisionBasedQuoteQuantityTruncator(m types.Market) QuantityTruncator {
	// note that MAX uses the price precision for its quote asset precision
	return createRoundedTruncator(m.PricePrecision)
}

func createRoundedTruncator(truncPrec int) QuantityTruncator {
	var truncPow10 = math.Pow10(truncPrec)
	var roundPrec = truncPrec + 1
	var roundPow10 = math.Pow10(roundPrec)
	return func(value fixedpoint.Value) fixedpoint.Value {
		v := value.Float64()
		v = math.Trunc(math.Round(v*roundPow10)/10.0) / truncPow10
		s := strconv.FormatFloat(v, 'f', truncPrec, 64)
		return fixedpoint.MustNewFromString(s)
	}
}

func (m *ArbMarket) getInitialBalance(balances types.BalanceMap, dir int) (fixedpoint.Value, string) {
	if dir == 1 { // sell 1 BTC -> 19000 USDT
		b, ok := balances[m.BaseCurrency]
		if !ok {
			return fixedpoint.Zero, m.BaseCurrency
		}

		return m.truncateBaseQuantity(b.Available), m.BaseCurrency
	} else if dir == -1 {
		b, ok := balances[m.QuoteCurrency]
		if !ok {
			return fixedpoint.Zero, m.QuoteCurrency
		}

		return m.truncateQuoteQuantity(b.Available), m.QuoteCurrency
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
		q, r := fitQuantityByBase(m.truncateBaseQuantity(m.bestBid.Volume).Float64(), transitingQuantity)
		return types.SubmitOrder{
			Symbol:   m.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimit,
			Quantity: fixedpoint.NewFromFloat(q),
			Price:    m.bestBid.Price,
			Market:   m.market,
		}, r
	} else if dir == -1 { // use 1 BTC to buy X ETH
		q, r := fitQuantityByQuote(m.bestAsk.Price.Float64(), m.truncateBaseQuantity(m.bestAsk.Volume).Float64(), transitingQuantity)
		return types.SubmitOrder{
			Symbol:   m.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Quantity: fixedpoint.NewFromFloat(q),
			Price:    m.bestAsk.Price,
			Market:   m.market,
		}, r
	} else {
		panic(fmt.Errorf("unexpected direction: %v, valid values are (1, -1)", dir))
	}

	return types.SubmitOrder{}, 0.0
}

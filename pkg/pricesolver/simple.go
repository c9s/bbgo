package pricesolver

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// SimplePriceSolver implements a map-structure-based price index
type SimplePriceSolver struct {
	// symbolPrices stores the latest trade price by mapping symbol to price
	symbolPrices map[string]fixedpoint.Value
	markets      types.MarketMap

	// pricesByBase stores the prices by currency names as a 2-level map
	// BTC -> USDT -> 48000.0
	// BTC -> TWD -> 1536000
	pricesByBase map[string]map[string]fixedpoint.Value

	// pricesByQuote is for reversed pairs, like USDT/TWD or BNB/BTC
	// the reason that we don't store the reverse pricing in the same map is:
	// expression like (1/price) could produce precision issue since the data type is fixed-point, only 8 fraction numbers are supported.
	pricesByQuote map[string]map[string]fixedpoint.Value

	mu sync.Mutex
}

func NewSimplePriceResolver(markets types.MarketMap) *SimplePriceSolver {
	return &SimplePriceSolver{
		markets:       markets,
		symbolPrices:  make(map[string]fixedpoint.Value),
		pricesByBase:  make(map[string]map[string]fixedpoint.Value),
		pricesByQuote: make(map[string]map[string]fixedpoint.Value),
	}
}

func (m *SimplePriceSolver) Update(symbol string, price fixedpoint.Value) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.symbolPrices[symbol] = price
	market, ok := m.markets[symbol]
	if !ok {
		log.Warnf("market info %s not found, unable to update price", symbol)
		return
	}

	quoteMap, ok2 := m.pricesByBase[market.BaseCurrency]
	if !ok2 {
		quoteMap = make(map[string]fixedpoint.Value)
		m.pricesByBase[market.BaseCurrency] = quoteMap
	}

	quoteMap[market.QuoteCurrency] = price

	baseMap, ok3 := m.pricesByQuote[market.QuoteCurrency]
	if !ok3 {
		baseMap = make(map[string]fixedpoint.Value)
		m.pricesByQuote[market.QuoteCurrency] = baseMap
	}

	baseMap[market.BaseCurrency] = price
}

func (m *SimplePriceSolver) UpdateFromTrade(trade types.Trade) {
	m.Update(trade.Symbol, trade.Price)
}

func (m *SimplePriceSolver) inferencePrice(asset string, assetPrice fixedpoint.Value, preferredFiats ...string) (fixedpoint.Value, bool) {
	// log.Infof("inferencePrice %s = %f", asset, assetPrice.Float64())
	quotePrices, ok := m.pricesByBase[asset]
	if ok {
		for quote, price := range quotePrices {
			for _, fiat := range preferredFiats {
				if quote == fiat {
					return price.Mul(assetPrice), true
				}
			}
		}

		for quote, price := range quotePrices {
			if infPrice, ok := m.inferencePrice(quote, price.Mul(assetPrice), preferredFiats...); ok {
				return infPrice, true
			}
		}
	}

	// for example, quote = TWD here, we can get a price map with:
	// USDT: 32.0 (for USDT/TWD at 32.0)
	basePrices, ok := m.pricesByQuote[asset]
	if ok {
		for base, basePrice := range basePrices {
			// log.Infof("base %s @ %s", base, basePrice.String())
			for _, fiat := range preferredFiats {
				if base == fiat {
					// log.Infof("ret %f / %f = %f", assetPrice.Float64(), basePrice.Float64(), assetPrice.Div(basePrice).Float64())
					return assetPrice.Div(basePrice), true
				}
			}
		}

		for base, basePrice := range basePrices {
			if infPrice, ok2 := m.inferencePrice(base, assetPrice.Div(basePrice), preferredFiats...); ok2 {
				return infPrice, true
			}
		}
	}

	return fixedpoint.Zero, false
}

func (m *SimplePriceSolver) ResolvePrice(asset string, preferredFiats ...string) (fixedpoint.Value, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inferencePrice(asset, fixedpoint.One, preferredFiats...)
}

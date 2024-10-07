package pricesolver

import (
	"context"
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
	pricesByBase map[string]map[string]float64

	// pricesByQuote is for reversed pairs, like USDT/TWD or BNB/BTC
	// the reason that we don't store the reverse pricing in the same map is:
	// expression like (1/price) could produce precision issue since the data type is fixed-point, only 8 fraction numbers are supported.
	pricesByQuote map[string]map[string]float64

	mu sync.Mutex
}

func NewSimplePriceResolver(markets types.MarketMap) *SimplePriceSolver {
	return &SimplePriceSolver{
		markets:       markets,
		symbolPrices:  make(map[string]fixedpoint.Value),
		pricesByBase:  make(map[string]map[string]float64),
		pricesByQuote: make(map[string]map[string]float64),
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
		quoteMap = make(map[string]float64)
		m.pricesByBase[market.BaseCurrency] = quoteMap
	}

	quoteMap[market.QuoteCurrency] = price.Float64()

	baseMap, ok3 := m.pricesByQuote[market.QuoteCurrency]
	if !ok3 {
		baseMap = make(map[string]float64)
		m.pricesByQuote[market.QuoteCurrency] = baseMap
	}

	baseMap[market.BaseCurrency] = price.Float64()
}

func (m *SimplePriceSolver) UpdateFromTrade(trade types.Trade) {
	m.Update(trade.Symbol, trade.Price)
}

func (m *SimplePriceSolver) BindStream(stream types.Stream) {
	stream.OnKLineClosed(func(k types.KLine) {
		m.Update(k.Symbol, k.Close)
	})
}

func (m *SimplePriceSolver) UpdateFromTickers(ctx context.Context, ex types.Exchange, symbols ...string) error {
	for _, symbol := range symbols {
		// only query the ticker for the symbol that is in the market map
		_, ok := m.markets[symbol]
		if !ok {
			continue
		}

		ticker, err := ex.QueryTicker(ctx, symbol)
		if err != nil {
			return err
		}

		price := ticker.GetValidPrice()
		if !price.IsZero() {
			m.Update(symbol, price)
		}
	}

	return nil
}

func (m *SimplePriceSolver) inferencePrice(asset string, assetPrice float64, preferredFiats ...string) (float64, bool) {
	quotePrices, ok := m.pricesByBase[asset]
	if ok {
		for quote, price := range quotePrices {
			for _, fiat := range preferredFiats {
				if quote == fiat {
					return price * assetPrice, true
				}
			}
		}

		for quote, price := range quotePrices {
			if infPrice, ok := m.inferencePrice(quote, price*assetPrice, preferredFiats...); ok {
				return infPrice, true
			}
		}
	}

	// for example, quote = TWD here, we can get a price map with:
	// USDT: 32.0 (for USDT/TWD at 32.0)
	basePrices, ok := m.pricesByQuote[asset]
	if ok {
		for base, basePrice := range basePrices {
			for _, fiat := range preferredFiats {
				if base == fiat {
					return assetPrice / basePrice, true
				}
			}
		}

		for base, basePrice := range basePrices {
			if infPrice, ok2 := m.inferencePrice(base, assetPrice/basePrice, preferredFiats...); ok2 {
				return infPrice, true
			}
		}
	}

	return 0.0, false
}

func (m *SimplePriceSolver) ResolvePrice(asset string, preferredFiats ...string) (fixedpoint.Value, bool) {
	if len(preferredFiats) == 0 {
		return fixedpoint.Zero, false
	} else if asset == preferredFiats[0] {
		return fixedpoint.One, true
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	fn, ok := m.inferencePrice(asset, 1.0, preferredFiats...)
	if ok {
		return fixedpoint.NewFromFloat(fn), ok
	}

	return fixedpoint.Zero, false
}

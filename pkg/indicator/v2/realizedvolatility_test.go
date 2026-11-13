package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

// tradesFromPrices builds a trade slice from a price series (BTCUSDT).
func tradesFromPrices(prices ...float64) []types.Trade {
	trades := make([]types.Trade, 0, len(prices))
	for i, p := range prices {
		trades = append(trades, makeTrade(uint64(i+1), "BTCUSDT", p))
	}
	return trades
}

func TestRealizedVolatility_coldReturnsZero(t *testing.T) {
	src := &TradeStream{}
	rv := RealizedVolatility(src, 0.94, 3)

	// no trades yet
	assert.Equal(t, 0.0, rv.Last(0))

	// fewer returns than warmup (3 prices => 2 returns) -> still cold
	src.BackFill(tradesFromPrices(100, 101, 100))
	assert.Equal(t, 0.0, rv.Last(0), "should not emit before warmup is reached")
}

func TestRealizedVolatility_warmsUpAndEmitsPositive(t *testing.T) {
	src := &TradeStream{}
	rv := RealizedVolatility(src, 0.94, 3)

	// 5 prices => 4 returns, all non-zero -> warm after the 3rd return
	src.BackFill(tradesFromPrices(100, 101, 100, 102, 101))

	assert.Positive(t, rv.Last(0), "indicator should emit a positive volatility once warm")
}

func TestRealizedVolatility_higherDispersionHigherVol(t *testing.T) {
	warmup := 3

	calm := &TradeStream{}
	rvCalm := RealizedVolatility(calm, 0.94, warmup)
	calm.BackFill(tradesFromPrices(100, 100.1, 100, 100.1, 100, 100.1))

	wild := &TradeStream{}
	rvWild := RealizedVolatility(wild, 0.94, warmup)
	wild.BackFill(tradesFromPrices(100, 105, 100, 105, 100, 105))

	assert.Positive(t, rvCalm.Last(0))
	assert.Positive(t, rvWild.Last(0))
	assert.Greater(t, rvWild.Last(0), rvCalm.Last(0),
		"a wider return dispersion must yield a higher realized volatility")
}

func TestRealizedVolatility_decaysWhenPriceIsFlat(t *testing.T) {
	// warmup=1 so we emit from the first return onward.
	src := &TradeStream{}
	rv := RealizedVolatility(src, 0.9, 1)

	// one volatility spike, then a flat price series (zero returns) that should
	// let the EWMA variance decay by lambda on each subsequent trade.
	src.BackFill(tradesFromPrices(100, 110)) // the spike
	peak := rv.Last(0)
	assert.Positive(t, peak)

	// flat price -> zero returns -> monotonically decreasing volatility
	src.BackFill(tradesFromPrices(110, 110, 110))
	after := rv.Last(0)
	assert.Less(t, after, peak, "volatility should decay while the price is flat")
}

func TestRealizedVolatility_ignoresNonPositivePrice(t *testing.T) {
	src := &TradeStream{}
	rv := RealizedVolatility(src, 0.94, 1)

	// a zero price must not produce a NaN/Inf return; it is skipped.
	src.BackFill(tradesFromPrices(100, 0, 101, 102))
	assert.False(t, isNaNOrInf(rv.Last(0)), "non-positive prices must be ignored, not poison the state")
	assert.Positive(t, rv.Last(0))
}

func isNaNOrInf(f float64) bool {
	return f != f || f > 1e308 || f < -1e308
}

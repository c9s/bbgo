package xmaker

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func newTestAdaptiveQuoteInterval() *AdaptiveQuoteInterval {
	a := &AdaptiveQuoteInterval{
		Enabled:        true,
		Lambda:         fixedpoint.NewFromFloat(0.94),
		Warmup:         3,
		LowVolatility:  fixedpoint.NewFromFloat(0.0005),
		HighVolatility: fixedpoint.NewFromFloat(0.005),
		MinInterval:    types.Duration(time.Second),
		MaxInterval:    types.Duration(time.Minute),
	}
	_ = a.Defaults()
	a.scale = a.buildScale()
	return a
}

func TestAdaptiveQuoteInterval_Defaults(t *testing.T) {
	a := &AdaptiveQuoteInterval{Enabled: true}
	assert.NoError(t, a.Defaults())

	assert.InDelta(t, 0.94, a.Lambda.Float64(), 1e-12)
	assert.Equal(t, 30, a.Warmup)
	assert.Equal(t, time.Second, a.MinInterval.Duration())
	assert.Equal(t, time.Minute, a.MaxInterval.Duration())
	assert.InDelta(t, 0.00005, a.LowVolatility.Float64(), 1e-12)
	assert.InDelta(t, 0.001, a.HighVolatility.Float64(), 1e-12)
}

func TestAdaptiveQuoteInterval_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		a := newTestAdaptiveQuoteInterval()
		assert.NoError(t, a.Validate())
	})

	t.Run("invalid lambda", func(t *testing.T) {
		a := newTestAdaptiveQuoteInterval()
		a.Lambda = fixedpoint.One
		assert.Error(t, a.Validate())

		a.Lambda = fixedpoint.Zero
		assert.Error(t, a.Validate())
	})

	t.Run("non-positive warmup", func(t *testing.T) {
		a := newTestAdaptiveQuoteInterval()
		a.Warmup = 0
		assert.Error(t, a.Validate())
	})

	t.Run("min greater than max", func(t *testing.T) {
		a := newTestAdaptiveQuoteInterval()
		a.MinInterval = types.Duration(2 * time.Minute)
		assert.Error(t, a.Validate())
	})

	t.Run("low not less than high", func(t *testing.T) {
		a := newTestAdaptiveQuoteInterval()
		a.LowVolatility = a.HighVolatility
		assert.Error(t, a.Validate())
	})
}

func TestAdaptiveQuoteInterval_intervalForVolatility(t *testing.T) {
	a := newTestAdaptiveQuoteInterval()
	fallback := 3 * time.Second

	t.Run("warm-up falls back", func(t *testing.T) {
		assert.Equal(t, fallback, a.intervalForVolatility(0, fallback))
		assert.Equal(t, fallback, a.intervalForVolatility(-1, fallback))
	})

	t.Run("low volatility clamps to max interval", func(t *testing.T) {
		// at or below LowVolatility -> MaxInterval (1m)
		assert.Equal(t, time.Minute, a.intervalForVolatility(0.0005, fallback))
		assert.Equal(t, time.Minute, a.intervalForVolatility(0.0001, fallback))
	})

	t.Run("high volatility clamps to min interval", func(t *testing.T) {
		// at or above HighVolatility -> MinInterval (1s)
		assert.Equal(t, time.Second, a.intervalForVolatility(0.005, fallback))
		assert.Equal(t, time.Second, a.intervalForVolatility(0.05, fallback))
	})

	t.Run("midpoint interpolates within bounds", func(t *testing.T) {
		mid := a.intervalForVolatility(0.00275, fallback) // halfway between 0.0005 and 0.005
		assert.Greater(t, mid, time.Second)
		assert.Less(t, mid, time.Minute)
	})

	t.Run("monotonically decreasing with volatility", func(t *testing.T) {
		// higher volatility -> shorter interval
		low := a.intervalForVolatility(0.001, fallback)
		high := a.intervalForVolatility(0.004, fallback)
		assert.Greater(t, low, high)
	})
}

// buildTrade constructs a synthetic trade at the given price.
func buildTrade(price float64) types.Trade {
	return types.Trade{Price: fixedpoint.NewFromFloat(price)}
}

func TestAdaptiveQuoteInterval_tradeWarmup(t *testing.T) {
	fallback := 3 * time.Second

	// mirror what Bind() wires up, but against a standalone trade stream so we can
	// drive the realized-volatility indicator directly without a live session.
	newWarmable := func() (*AdaptiveQuoteInterval, *indicatorv2.TradeStream) {
		a := newTestAdaptiveQuoteInterval()
		ts := &indicatorv2.TradeStream{}
		a.rv = indicatorv2.RealizedVolatility(ts, a.Lambda.Float64(), a.Warmup)
		return a, ts
	}

	t.Run("cold stream falls back", func(t *testing.T) {
		a, _ := newWarmable()
		// no trades fed -> volatility is non-positive -> fallback
		assert.Equal(t, fallback, a.NextInterval(fallback))
	})

	t.Run("trades warm up the indicator", func(t *testing.T) {
		a, ts := newWarmable()

		// feed enough volatile trades (alternating ~1% around 100) so the realized
		// volatility produces a positive value and NextInterval no longer returns
		// the fallback.
		var trades []types.Trade
		for i := 0; i < a.Warmup*4; i++ {
			price := 99.0
			if i%2 == 0 {
				price = 101.0
			}
			trades = append(trades, buildTrade(price))
		}
		ts.BackFill(trades)

		next := a.NextInterval(fallback)
		assert.NotEqual(t, fallback, next, "indicator should be warm after trades")
		assert.GreaterOrEqual(t, next, a.MinInterval.Duration())
		assert.LessOrEqual(t, next, a.MaxInterval.Duration())
	})
}

func TestAdaptiveQuoteInterval_ConfigParse(t *testing.T) {
	const payload = `{
		"enabled": true,
		"lambda": 0.9,
		"warmup": 50,
		"minInterval": "1s",
		"maxInterval": "1m",
		"lowVolatility": 0.0001,
		"highVolatility": 0.002
	}`

	var a AdaptiveQuoteInterval
	assert.NoError(t, json.Unmarshal([]byte(payload), &a))

	assert.True(t, a.Enabled)
	assert.InDelta(t, 0.9, a.Lambda.Float64(), 1e-12)
	assert.Equal(t, 50, a.Warmup)
	assert.Equal(t, time.Second, a.MinInterval.Duration())
	assert.Equal(t, time.Minute, a.MaxInterval.Duration())
	assert.InDelta(t, 0.0001, a.LowVolatility.Float64(), 1e-12)
	assert.InDelta(t, 0.002, a.HighVolatility.Float64(), 1e-12)
	assert.NoError(t, a.Validate())
}

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

	assert.Equal(t, types.Interval1m, a.Interval)
	assert.Equal(t, 14, a.Window)
	assert.Equal(t, time.Second, a.MinInterval.Duration())
	assert.Equal(t, time.Minute, a.MaxInterval.Duration())
	assert.InDelta(t, 0.0005, a.LowVolatility.Float64(), 1e-12)
	assert.InDelta(t, 0.005, a.HighVolatility.Float64(), 1e-12)
}

func TestAdaptiveQuoteInterval_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		a := newTestAdaptiveQuoteInterval()
		assert.NoError(t, a.Validate())
	})

	t.Run("non-positive window", func(t *testing.T) {
		a := newTestAdaptiveQuoteInterval()
		a.Window = 0
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

// buildKLine constructs a synthetic kline with the given OHLC values.
func buildKLine(high, low, cloze float64) types.KLine {
	return types.KLine{
		High:  fixedpoint.NewFromFloat(high),
		Low:   fixedpoint.NewFromFloat(low),
		Open:  fixedpoint.NewFromFloat(cloze),
		Close: fixedpoint.NewFromFloat(cloze),
	}
}

func TestAdaptiveQuoteInterval_backfillWarmup(t *testing.T) {
	fallback := 3 * time.Second

	// mirror what Bind() wires up, but against a standalone kline stream so we can
	// drive the ATR indicator directly without a live session.
	newWarmable := func() (*AdaptiveQuoteInterval, *indicatorv2.KLineStream) {
		a := newTestAdaptiveQuoteInterval()
		ks := &indicatorv2.KLineStream{}
		a.atrp = indicatorv2.ATRP2(ks, a.Window)
		return a, ks
	}

	t.Run("cold stream falls back", func(t *testing.T) {
		a, _ := newWarmable()
		// no klines fed -> atrp is non-positive -> fallback
		assert.Equal(t, fallback, a.NextInterval(fallback))
	})

	t.Run("backfilled klines warm up the indicator", func(t *testing.T) {
		a, ks := newWarmable()

		// feed enough volatile klines (~1% range around 100) so the ATRP produces a
		// positive value and NextInterval no longer returns the fallback.
		var kLines []types.KLine
		for i := 0; i < a.Window*4; i++ {
			kLines = append(kLines, buildKLine(100.5, 99.5, 100.0))
		}
		ks.BackFill(kLines)

		next := a.NextInterval(fallback)
		assert.NotEqual(t, fallback, next, "indicator should be warm after backfill")
		assert.GreaterOrEqual(t, next, a.MinInterval.Duration())
		assert.LessOrEqual(t, next, a.MaxInterval.Duration())
	})
}

func TestAdaptiveQuoteInterval_ConfigParse(t *testing.T) {
	const payload = `{
		"enabled": true,
		"interval": "5m",
		"window": 20,
		"minInterval": "1s",
		"maxInterval": "1m",
		"lowVolatility": 0.0008,
		"highVolatility": 0.006
	}`

	var a AdaptiveQuoteInterval
	assert.NoError(t, json.Unmarshal([]byte(payload), &a))

	assert.True(t, a.Enabled)
	assert.Equal(t, types.Interval5m, a.Interval)
	assert.Equal(t, 20, a.Window)
	assert.Equal(t, time.Second, a.MinInterval.Duration())
	assert.Equal(t, time.Minute, a.MaxInterval.Duration())
	assert.InDelta(t, 0.0008, a.LowVolatility.Float64(), 1e-12)
	assert.InDelta(t, 0.006, a.HighVolatility.Float64(), 1e-12)
	assert.NoError(t, a.Validate())
}

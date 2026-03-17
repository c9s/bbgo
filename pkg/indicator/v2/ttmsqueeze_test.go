package indicatorv2

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func makeTTMKLine(ts time.Time, open, high, low, close float64) types.KLine {
	return types.KLine{
		StartTime: types.Time(ts),
		EndTime:   types.Time(ts.Add(time.Minute)),
		Open:      fixedpoint.NewFromFloat(open),
		High:      fixedpoint.NewFromFloat(high),
		Low:       fixedpoint.NewFromFloat(low),
		Close:     fixedpoint.NewFromFloat(close),
	}
}

// TestTTMSqueezeStream_Emits verifies the emission boundary around the window size.
func TestTTMSqueezeStream_Emits(t *testing.T) {
	const window = 5
	now := time.Now()

	t.Run("NoEmissionBeforeWindowFilled", func(t *testing.T) {
		stdStream := &types.StandardStream{}
		klineStream := KLines(stdStream, "", "")
		ttm := NewTTMSqueezeStream(klineStream, window)

		var updates []TTMSqueeze
		ttm.OnUpdate(func(s TTMSqueeze) {
			updates = append(updates, s)
		})

		for i := 0; i < window-1; i++ {
			p := float64(100 + i)
			stdStream.EmitKLineClosed(makeTTMKLine(now.Add(time.Duration(i)*time.Minute), p, p+1, p-1, p))
		}

		assert.Empty(t, updates, "should not emit before window klines are accumulated")
	})

	t.Run("EmitsAfterWindowFilled", func(t *testing.T) {
		stdStream := &types.StandardStream{}
		klineStream := KLines(stdStream, "", "")
		ttm := NewTTMSqueezeStream(klineStream, window)

		var updates []TTMSqueeze
		ttm.OnUpdate(func(s TTMSqueeze) {
			updates = append(updates, s)
		})

		for i := 0; i < window; i++ {
			p := float64(100 + i)
			stdStream.EmitKLineClosed(makeTTMKLine(now.Add(time.Duration(i)*time.Minute), p, p+1, p-1, p))
		}

		assert.NotEmpty(t, updates, "should emit after window klines are accumulated")
	})
}

// TestTTMSqueezeStream_MomentumDirection verifies that momentum sign and
// direction agree with the trend of the input prices.
func TestTTMSqueezeStream_MomentumDirection(t *testing.T) {
	const window = 5
	now := time.Now()

	t.Run("Bullish", func(t *testing.T) {
		stdStream := &types.StandardStream{}
		klineStream := KLines(stdStream, "", "")
		ttm := NewTTMSqueezeStream(klineStream, window)

		var last TTMSqueeze
		ttm.OnUpdate(func(s TTMSqueeze) { last = s })

		for i := 0; i < window*3; i++ {
			p := float64(100 + i)
			stdStream.EmitKLineClosed(makeTTMKLine(now.Add(time.Duration(i)*time.Minute), p, p+0.5, p-0.5, p))
		}

		assert.Positive(t, last.Momentum, "momentum should be positive for ascending prices")
		assert.True(t,
			last.MomentumDirection == MomentumDirectionBullish ||
				last.MomentumDirection == MomentumDirectionBullishSlowing,
			"direction should be bullish for ascending prices, got %v", last.MomentumDirection,
		)
	})

	t.Run("Bearish", func(t *testing.T) {
		stdStream := &types.StandardStream{}
		klineStream := KLines(stdStream, "", "")
		ttm := NewTTMSqueezeStream(klineStream, window)

		var last TTMSqueeze
		ttm.OnUpdate(func(s TTMSqueeze) { last = s })

		for i := 0; i < window*3; i++ {
			p := float64(200 - i)
			stdStream.EmitKLineClosed(makeTTMKLine(now.Add(time.Duration(i)*time.Minute), p, p+0.5, p-0.5, p))
		}

		assert.Negative(t, last.Momentum, "momentum should be negative for descending prices")
		assert.True(t,
			last.MomentumDirection == MomentumDirectionBearish ||
				last.MomentumDirection == MomentumDirectionBearishSlowing,
			"direction should be bearish for descending prices, got %v", last.MomentumDirection,
		)
	})

	t.Run("InRange", func(t *testing.T) {
		stdStream := &types.StandardStream{}
		klineStream := KLines(stdStream, "", "")
		ttm := NewTTMSqueezeStream(klineStream, window)

		var updates []TTMSqueeze
		ttm.OnUpdate(func(s TTMSqueeze) { updates = append(updates, s) })

		for i := 0; i < window*4; i++ {
			p := float64(100 + i%10)
			stdStream.EmitKLineClosed(makeTTMKLine(now.Add(time.Duration(i)*time.Minute), p, p+2, p-2, p))
		}

		assert.NotEmpty(t, updates)
		for _, u := range updates {
			assert.GreaterOrEqual(t, u.MomentumDirection, MomentumDirectionNeutral)
			assert.LessOrEqual(t, u.MomentumDirection, MomentumDirectionBearishSlowing)
		}
	})
}

// TestTTMSqueezeStream_CompressionLevel verifies compression level behaviour.
func TestTTMSqueezeStream_CompressionLevel(t *testing.T) {
	const window = 5
	now := time.Now()

	// HighOnFlatPrices verifies that flat (zero volatility) candles result in
	// the highest compression level, since both BB and ATR collapse to zero,
	// making BB width ≤ KC width at every multiplier.
	t.Run("HighOnFlatPrices", func(t *testing.T) {
		stdStream := &types.StandardStream{}
		klineStream := KLines(stdStream, "", "")
		ttm := NewTTMSqueezeStream(klineStream, window)

		var last TTMSqueeze
		ttm.OnUpdate(func(s TTMSqueeze) { last = s })

		for i := 0; i < window*2; i++ {
			stdStream.EmitKLineClosed(makeTTMKLine(now.Add(time.Duration(i)*time.Minute), 100, 100, 100, 100))
		}

		assert.Equal(t, CompressionLevelHigh, last.CompressionLevel,
			"constant prices should produce high compression level")
	})

	// InRange verifies that every emitted CompressionLevel value falls within
	// the defined constant range.
	t.Run("InRange", func(t *testing.T) {
		stdStream := &types.StandardStream{}
		klineStream := KLines(stdStream, "", "")
		ttm := NewTTMSqueezeStream(klineStream, window)

		var updates []TTMSqueeze
		ttm.OnUpdate(func(s TTMSqueeze) { updates = append(updates, s) })

		for i := 0; i < window*4; i++ {
			p := float64(100 + i%10)
			stdStream.EmitKLineClosed(makeTTMKLine(now.Add(time.Duration(i)*time.Minute), p, p+2, p-2, p))
		}

		assert.NotEmpty(t, updates)
		for _, u := range updates {
			assert.GreaterOrEqual(t, u.CompressionLevel, CompressionLevelNone)
			assert.LessOrEqual(t, u.CompressionLevel, CompressionLevelHigh)
		}
	})
}

// TestTTMSqueezeStream_TimestampMatchesKLine verifies that every emitted
// TTMSqueeze carries the StartTime of the corresponding input kline.
func TestTTMSqueezeStream_TimestampMatchesKLine(t *testing.T) {
	const window = 5
	stdStream := &types.StandardStream{}
	klineStream := KLines(stdStream, "", "")
	ttm := NewTTMSqueezeStream(klineStream, window)

	var updates []TTMSqueeze
	ttm.OnUpdate(func(s TTMSqueeze) { updates = append(updates, s) })

	now := time.Now().Truncate(time.Second)
	var times []time.Time
	for i := 0; i < window*2; i++ {
		ts := now.Add(time.Duration(i) * time.Minute)
		p := float64(100 + i)
		stdStream.EmitKLineClosed(makeTTMKLine(ts, p, p+1, p-1, p))
		if i >= window-1 {
			times = append(times, ts)
		}
	}

	assert.Equal(t, len(times), len(updates))
	for i, u := range updates {
		assert.Equal(t, times[i].Unix(), u.Time.Time().Unix(),
			"update timestamp should match kline StartTime")
	}
}

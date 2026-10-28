package grid2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestAutoRangeFromPivots(t *testing.T) {
	symbol := "BTCUSDT"
	interval := types.Interval1h
	iw := types.IntervalWindow{Interval: interval, Window: 2}

	highs := []float64{1, 2, 5, 2, 1, 3, 4, 3, 2}
	lows := []float64{0.5, 0.8, 0.3, 0.9, 1.0, 0.6, 0.4, 0.7, 0.8}

	t.Run("no history", func(t *testing.T) {
		store := types.NewMarketDataStore(symbol)
		indicators := bbgo.NewIndicatorSet(symbol, &types.StandardStream{}, store)
		upper, lower := autoRangeFromPivots(indicators, iw)
		assert.True(t, upper.IsZero())
		assert.True(t, lower.IsZero())
	})

	t.Run("last pivots of the history", func(t *testing.T) {
		store := types.NewMarketDataStore(symbol)
		startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := range highs {
			st := startTime.Add(time.Duration(i) * interval.Duration())
			store.AddKLine(types.KLine{
				Symbol:    symbol,
				Interval:  interval,
				StartTime: types.Time(st),
				EndTime:   types.Time(st.Add(interval.Duration() - time.Millisecond)),
				Open:      fixedpoint.NewFromFloat(lows[i]),
				Close:     fixedpoint.NewFromFloat(highs[i]),
				High:      fixedpoint.NewFromFloat(highs[i]),
				Low:       fixedpoint.NewFromFloat(lows[i]),
				Closed:    true,
			})
		}

		indicators := bbgo.NewIndicatorSet(symbol, &types.StandardStream{}, store)
		upper, lower := autoRangeFromPivots(indicators, iw)

		// the pivot highs are 5 and 4, the pivot lows are 0.3 and 0.4, the latest ones are used
		assert.InDelta(t, 4.0, upper.Float64(), 1e-9)
		assert.InDelta(t, 0.4, lower.Float64(), 1e-9)
	})
}

func TestLastPositive(t *testing.T) {
	assert.Equal(t, 0.0, lastPositive(types.NewFloat64Series()))
	assert.Equal(t, 2.0, lastPositive(types.NewFloat64Series(1, 2, 0, -1)))
}

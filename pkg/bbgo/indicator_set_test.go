package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func newTestIndicatorSet() *IndicatorSet {
	symbol := "BTCUSDT"
	store := NewMarketDataStore(symbol)
	store.KLineWindows[types.Interval1m] = &types.KLineWindow{
		{Open: number(19000.0), Close: number(19100.0)},
		{Open: number(19100.0), Close: number(19200.0)},
		{Open: number(19200.0), Close: number(19300.0)},
		{Open: number(19300.0), Close: number(19200.0)},
		{Open: number(19200.0), Close: number(19100.0)},
		{Open: number(19100.0), Close: number(19500.0)},
		{Open: number(19500.0), Close: number(19600.0)},
		{Open: number(19600.0), Close: number(19700.0)},
	}

	stream := types.NewStandardStream()
	indicatorSet := NewIndicatorSet(symbol, &stream, store)
	return indicatorSet
}

func TestIndicatorSet_closeCache(t *testing.T) {
	indicatorSet := newTestIndicatorSet()

	close1m := indicatorSet.CLOSE(types.Interval1m)
	assert.NotNil(t, close1m)

	close1m2 := indicatorSet.CLOSE(types.Interval1m)
	assert.Equal(t, close1m, close1m2)
}

func TestIndicatorSet_RSI(t *testing.T) {
	indicatorSet := newTestIndicatorSet()

	rsi1m := indicatorSet.RSI(types.IntervalWindow{Interval: types.Interval1m, Window: 7})
	assert.NotNil(t, rsi1m)

	rsiLast := rsi1m.Last(0)
	assert.InDelta(t, 80, rsiLast, 0.0000001)
}

func TestIndicatorSet_EWMA(t *testing.T) {
	indicatorSet := newTestIndicatorSet()

	ema1m := indicatorSet.EWMA(types.IntervalWindow{Interval: types.Interval1m, Window: 7})
	assert.NotNil(t, ema1m)

	emaLast := ema1m.Last(0)
	assert.InDelta(t, 19424.224853515625, emaLast, 0.0000001)
}

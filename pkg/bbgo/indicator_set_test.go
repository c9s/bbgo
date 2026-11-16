package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func newTestIndicatorSet() *IndicatorSet {
	symbol := "BTCUSDT"
	store := types.NewMarketDataStore(symbol)
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

func TestIndicatorSet_Aroon(t *testing.T) {
	symbol := "BTCUSDT"
	store := types.NewMarketDataStore(symbol)
	store.KLineWindows[types.Interval1m] = &types.KLineWindow{
		{High: number(20.0), Low: number(10.0)},
		{High: number(22.0), Low: number(12.0)},
		{High: number(25.0), Low: number(15.0)},
		{High: number(23.0), Low: number(13.0)},
		{High: number(21.0), Low: number(11.0)},
		{High: number(19.0), Low: number(9.0)},
	}

	stream := types.NewStandardStream()
	indicatorSet := NewIndicatorSet(symbol, &stream, store)

	aroon := indicatorSet.Aroon(types.Interval1m, 5)
	assert.NotNil(t, aroon)
	assert.Equal(t, 1, aroon.Length())
	assert.InDelta(t, 40.0, aroon.Up.Last(0), 1e-4)
	assert.InDelta(t, 100.0, aroon.Down.Last(0), 1e-4)
	assert.InDelta(t, -60.0, aroon.Oscillator.Last(0), 1e-4)
	assert.InDelta(t, -60.0, aroon.Last(0), 1e-4)
}


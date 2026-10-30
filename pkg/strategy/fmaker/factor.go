package fmaker

import (
	"time"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

var zeroTime time.Time

type KLineValueMapper func(k types.KLine) float64

// kLineWindowFactor is a factor stream calculated from the last `window` klines of its source.
//
// The factor reads the kline window with source.Last(i) from inside its subscriber,
// so the source must emit each kline after it has been appended to it, kline by kline
// (e.g. a stream built with indicatorv2.WithHistory or filled with BackFill).
//
// No value is emitted until the source has at least `window` klines.
type kLineWindowFactor struct {
	*types.Float64Series

	window  int
	source  indicatorv2.KLineSubscription
	endTime time.Time

	name      string
	calculate func(kLines []types.KLine) (float64, error)
}

func newKLineWindowFactor(
	name string, source indicatorv2.KLineSubscription, window int, calculate func(kLines []types.KLine) (float64, error),
) *kLineWindowFactor {
	f := &kLineWindowFactor{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		source:        source,
		name:          name,
		calculate:     calculate,
	}
	source.AddSubscriber(f.handleKLine)
	return f
}

func (f *kLineWindowFactor) handleKLine(k types.KLine) {
	if f.source.Length() < f.window {
		return
	}

	if f.endTime != zeroTime && k.GetEndTime().Before(f.endTime) {
		return
	}

	// the last `window` klines, oldest first
	recentT := make([]types.KLine, 0, f.window)
	for i := f.window - 1; i >= 0; i-- {
		recentT = append(recentT, *f.source.Last(i))
	}

	val, err := f.calculate(recentT)
	if err != nil {
		log.WithError(err).Errorf("can not calculate %s", f.name)
		return
	}

	f.Slice.Push(val)
	f.Slice = types.ShrinkSlice(f.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
	f.endTime = k.GetEndTime().Time()
	f.EmitUpdate(val)
}

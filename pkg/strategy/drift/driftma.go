package drift

import (
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// DriftMA smooths the source with an EWMA, computes the volume weighted drift of the smoothed values,
// and filters the drift with a Fisher Transform.
//
// It is fed manually through Update, the embedded series holds the Fisher Transform output.
type DriftMA struct {
	*types.Float64Series

	input *types.Float64Series
	ma1   *indicatorv2.EWMAStream
	drift *indicatorv2.WeightedDriftStream
	ma2   *indicatorv2.FisherTransformStream
}

func NewDriftMA(driftWindow, smootherWindow, fisherTransformWindow int) *DriftMA {
	input := types.NewFloat64Series()
	ma1 := indicatorv2.EWMA2(input, smootherWindow)
	// the drift stream is fed through Update, so it is built on an empty kline stream that never emits.
	drift := indicatorv2.WeightedDrift(&indicatorv2.KLineStream{}, driftWindow)
	ma2 := indicatorv2.FisherTransform(drift, fisherTransformWindow)
	return &DriftMA{
		Float64Series: ma2.Float64Series,
		input:         input,
		ma1:           ma1,
		drift:         drift,
		ma2:           ma2,
	}
}

func (s *DriftMA) Update(value, weight float64) {
	pushFloat64(s.input, value)
	if s.ma1.Length() == 0 {
		return
	}
	// the drift stream emits only when its window is filled, which updates ma2
	s.drift.Update(s.ma1.Last(0), weight)
}

func (s *DriftMA) ZeroPoint() float64 {
	return s.drift.ZeroPoint()
}

// pushFloat64 pushes the value into a manually fed source series, emits it to the subscribers,
// and keeps the source series bounded.
func pushFloat64(s *types.Float64Series, v float64) {
	s.PushAndEmit(v)
	s.Slice = types.ShrinkSlice(s.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
}

// klineFeed is a KLineSubscription that is fed manually by the strategy.
// Unlike indicatorv2.KLineStream, it only keeps the last kline, so it does not grow unbounded.
type klineFeed struct {
	subscribers []func(k types.KLine)
	last        *types.KLine
	length      int
}

func (f *klineFeed) AddSubscriber(fn func(k types.KLine)) {
	f.subscribers = append(f.subscribers, fn)
}

func (f *klineFeed) Length() int {
	return f.length
}

func (f *klineFeed) Last(i int) *types.KLine {
	if i != 0 {
		return nil
	}
	return f.last
}

// Push emits the kline to the subscribers.
// Like the legacy indicator PushK, a kline that does not end after the last kline is ignored.
func (f *klineFeed) Push(k types.KLine) {
	if f.last != nil && !f.last.EndTime.Time().IsZero() && !k.EndTime.Time().After(f.last.EndTime.Time()) {
		return
	}

	f.last = &k
	f.length++
	for _, fn := range f.subscribers {
		fn(k)
	}
}

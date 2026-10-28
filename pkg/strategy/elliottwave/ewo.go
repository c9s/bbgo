package elliottwave

import (
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// ElliottWave is the Elliott Wave Oscillator: maQuick / maSlow - 1.
//
// It is fed manually through Update. The v2 SMA emits partial window means from the first value,
// so ElliottWave only exposes the values whose SMA windows are filled, like the legacy SMA did.
type ElliottWave struct {
	input   *types.Float64Series
	maSlow  *indicatorv2.SMAStream
	maQuick *indicatorv2.SMAStream

	windowSlow, windowQuick int

	// count is the number of the values pushed
	count int
}

func NewElliottWave(windowQuick, windowSlow int) *ElliottWave {
	input := types.NewFloat64Series()
	return &ElliottWave{
		input:       input,
		maSlow:      indicatorv2.SMA(input, windowSlow),
		maQuick:     indicatorv2.SMA(input, windowQuick),
		windowSlow:  windowSlow,
		windowQuick: windowQuick,
	}
}

// filledLength returns the number of the values of sma computed with a filled window
func (s *ElliottWave) filledLength(sma *indicatorv2.SMAStream, window int) int {
	return max(0, min(sma.Length(), s.count-window+1))
}

// filledIndex returns the i-th last value of sma computed with a filled window, or 0 if there is none
func (s *ElliottWave) filledIndex(sma *indicatorv2.SMAStream, window, i int) float64 {
	if i < 0 || i >= s.filledLength(sma, window) {
		return 0
	}
	return sma.Last(i)
}

func (s *ElliottWave) Index(i int) float64 {
	return s.Last(i)
}

func (s *ElliottWave) Last(i int) float64 {
	return s.filledIndex(s.maQuick, s.windowQuick, i)/s.filledIndex(s.maSlow, s.windowSlow, i) - 1.0
}

func (s *ElliottWave) Length() int {
	return s.filledLength(s.maSlow, s.windowSlow)
}

func (s *ElliottWave) Update(v float64) {
	s.count++
	s.input.PushAndEmit(v)
	s.input.Slice = types.ShrinkSlice(s.input.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
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

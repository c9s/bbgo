package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// AroonStream calculates the Aroon Up, Aroon Down, and Aroon Oscillator indicators.
//
// The Aroon indicator was developed by Tushar Chande in 1995 to identify whether a security
// is trending and the strength of the trend.
//
// Aroon Up = ((period - periods since highest high) / period) * 100
// Aroon Down = ((period - periods since lowest low) / period) * 100
// Aroon Oscillator = Aroon Up - Aroon Down
//
// When highest high occurs on the current bar, periods since highest high is 0, giving Aroon Up = 100.
// When ties occur within the lookback window, the most recent occurrence (closest to current bar) is used.
type AroonStream struct {
	types.SeriesBase

	Up         *types.Float64Series
	Down       *types.Float64Series
	Oscillator *types.Float64Series

	window int
	highs  floats.Slice
	lows   floats.Slice

	updateCallbacks []func(up, down, osc float64)
}

func Aroon(source KLineSubscription, window int) *AroonStream {
	checkWindow(window)

	s := &AroonStream{
		Up:         types.NewFloat64Series(),
		Down:       types.NewFloat64Series(),
		Oscillator: types.NewFloat64Series(),
		window:     window,
	}
	s.SeriesBase.Series = s.Oscillator

	source.AddSubscriber(func(k types.KLine) {
		s.calculateAndPush(k.High.Float64(), k.Low.Float64())
	})

	return s
}

func (s *AroonStream) calculateAndPush(high, low float64) {
	s.highs.Push(high)
	s.lows.Push(low)

	// We need window + 1 bars (current bar plus window prior bars) to compute Aroon
	if s.highs.Length() < s.window+1 {
		return
	}

	length := s.highs.Length()
	// Examine the last window + 1 elements: from index (length - 1 - window) to (length - 1)
	// periodsAgo = 0 is the current bar (length - 1)
	// periodsAgo = window is the oldest bar in the window (length - 1 - window)
	highestHigh := s.highs[length-1]
	periodsSinceHigh := 0

	lowestLow := s.lows[length-1]
	periodsSinceLow := 0

	for periodsAgo := 1; periodsAgo <= s.window; periodsAgo++ {
		idx := length - 1 - periodsAgo
		h := s.highs[idx]
		l := s.lows[idx]

		// Using strict > and < ensures that if there's a tie, the more recent
		// bar (smaller periodsAgo) retains precedence, adhering to TA-Lib standards.
		if h > highestHigh {
			highestHigh = h
			periodsSinceHigh = periodsAgo
		}
		if l < lowestLow {
			lowestLow = l
			periodsSinceLow = periodsAgo
		}
	}

	w := float64(s.window)
	up := float64(s.window-periodsSinceHigh) / w * 100.0
	down := float64(s.window-periodsSinceLow) / w * 100.0
	osc := up - down

	s.Up.PushAndEmit(up)
	s.Down.PushAndEmit(down)
	s.Oscillator.PushAndEmit(osc)

	s.EmitUpdate(up, down, osc)
	s.Truncate()
}

func (s *AroonStream) OnUpdate(cb func(up, down, osc float64)) {
	s.updateCallbacks = append(s.updateCallbacks, cb)
}

func (s *AroonStream) EmitUpdate(up, down, osc float64) {
	for _, cb := range s.updateCallbacks {
		cb(up, down, osc)
	}
}

func (s *AroonStream) Truncate() {
	s.Up.Slice = generalTruncate(s.Up.Slice)
	s.Down.Slice = generalTruncate(s.Down.Slice)
	s.Oscillator.Slice = generalTruncate(s.Oscillator.Slice)

	if s.highs.Length() > MaxSliceSize+s.window+1 {
		s.highs = s.highs[s.highs.Length()-(MaxSliceSize+s.window+1):]
		s.lows = s.lows[s.lows.Length()-(MaxSliceSize+s.window+1):]
	}
}

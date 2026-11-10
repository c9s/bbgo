package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Aroon Indicator
// - https://www.investopedia.com/terms/a/aroon.asp
//
// The Aroon indicator was developed by Tushar Chande in 1995 to identify trend changes
// and determine trend strength over a period n (standard default 14 or 25).
//
// Formulas:
// Aroon Up = ((period - periods_since_highest_high) / period) * 100
// Aroon Down = ((period - periods_since_lowest_low) / period) * 100
// Aroon Oscillator = Aroon Up - Aroon Down
//
// Values for Aroon Up and Aroon Down range from 0 to 100.
// Values for Aroon Oscillator range from -100 to +100.

//go:generate callbackgen -type AroonStream -output aroonstream_callbacks.go
type AroonStream struct {
	types.SeriesBase

	Up         *types.Float64Series
	Down       *types.Float64Series
	Oscillator *types.Float64Series

	window int

	highPrices *PriceStream
	lowPrices  *PriceStream

	updateCallbacks []func(up, down float64)
}

var _ types.SeriesExtend = &AroonStream{}

// Aroon constructs an AroonStream subscribing to the provided KLineSubscription.
func Aroon(source KLineSubscription, window int) *AroonStream {
	checkWindow(window)

	highPrices := HighPrices(source)
	lowPrices := LowPrices(source)

	s := &AroonStream{
		Up:         types.NewFloat64Series(),
		Down:       types.NewFloat64Series(),
		Oscillator: types.NewFloat64Series(),
		window:     window,
		highPrices: highPrices,
		lowPrices:  lowPrices,
	}
	s.SeriesBase.Series = s.Oscillator

	if source != nil {
		source.AddSubscriber(func(kLine types.KLine) {
			up, down := s.calculate()
			osc := up - down

			s.Up.PushAndEmit(up)
			s.Down.PushAndEmit(down)
			s.Oscillator.PushAndEmit(osc)

			s.Truncate()
			s.EmitUpdate(up, down)
		})
	}

	return s
}

func (s *AroonStream) calculate() (up, down float64) {
	if s.window <= 0 || len(s.highPrices.Slice) == 0 {
		return 0.0, 0.0
	}

	tailHigh := s.highPrices.Slice.Tail(s.window + 1)
	tailLow := s.lowPrices.Slice.Tail(s.window + 1)
	length := len(tailHigh)
	if length == 0 {
		return 0.0, 0.0
	}

	// Current bar is at index length - 1 (offset 0 from current bar).
	// When multiple bars share the same extreme value, the most recent bar is selected.
	highest := tailHigh[length-1]
	highestIdx := length - 1
	for i := length - 2; i >= 0; i-- {
		if tailHigh[i] > highest {
			highest = tailHigh[i]
			highestIdx = i
		}
	}

	lowest := tailLow[length-1]
	lowestIdx := length - 1
	for i := length - 2; i >= 0; i-- {
		if tailLow[i] < lowest {
			lowest = tailLow[i]
			lowestIdx = i
		}
	}

	periodsSinceHigh := float64(length - 1 - highestIdx)
	periodsSinceLow := float64(length - 1 - lowestIdx)

	period := float64(s.window)
	up = ((period - periodsSinceHigh) / period) * 100.0
	down = ((period - periodsSinceLow) / period) * 100.0
	return up, down
}

func (s *AroonStream) Truncate() {
	s.Up.Slice = generalTruncate(s.Up.Slice)
	s.Down.Slice = generalTruncate(s.Down.Slice)
	s.Oscillator.Slice = generalTruncate(s.Oscillator.Slice)
}

func (s *AroonStream) LastUp() float64 {
	return s.Up.Last(0)
}

func (s *AroonStream) LastDown() float64 {
	return s.Down.Last(0)
}

func (s *AroonStream) LastOscillator() float64 {
	return s.Oscillator.Last(0)
}

func (s *AroonStream) AroonUp() *types.Float64Series {
	return s.Up
}

func (s *AroonStream) AroonDown() *types.Float64Series {
	return s.Down
}

func (s *AroonStream) AroonOscillator() *types.Float64Series {
	return s.Oscillator
}

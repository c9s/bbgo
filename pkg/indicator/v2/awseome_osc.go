package indicatorv2

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

// https://github.com/Nikhil-Adithyan/Algorithmic-Trading-with-Awesome-Oscillator-in-Python/blob/master/Strategy_code.py
type AwesomeOscillatorStream struct {
	// embedded structs
	*types.Float64Series
	sma5  *SMAStream
	sma34 *SMAStream
}

// Awesome Oscillator.
//
// Median Price = ((Low + High) / 2) >> need HL2Source
// AO = 5-Period SMA - 34-Period SMA.
//
// Returns ao.
func AwesomeOscillator(source types.Float64Source) *AwesomeOscillatorStream {
	s := &AwesomeOscillatorStream{
		Float64Series: types.NewFloat64Series(),
		sma5:          SMA(source, 5),
		sma34:         SMA(source, 34),
	}
	s.Bind(source, s)
	return s
}

func (s *AwesomeOscillatorStream) Calculate(v float64) float64 {
	if s.Length() < 33 {
		return 0
	}
	ao := 0.0
	if s.Slice.Length() > 0 {
		ao = s.sma5.Last(0) - s.sma34.Last(0)

		var (
			prevao     = s.sma5.Last(0) - s.sma34.Last(0)
			currDiff   = ao - v
			prevDiff   = prevao - s.Slice.Last(0)
			crossOver  = crossOver(currDiff, prevDiff, 0)
			crossUnder = crossUnder(currDiff, prevDiff, 0)
		)
		if crossOver {
			fmt.Println("awesome oscillator changed to green: ", ao)
		}
		if crossUnder {
			fmt.Println("awesome oscillator changed to red: ", ao)
		}
	}

	return ao
}

func (s *AwesomeOscillatorStream) Truncate() {
	s.Slice = s.Slice.Truncate(5000)
}

// todo move this
// CrossOver returns true if the latest series values cross above x
func crossOver(curr, prev, x float64) bool {
	if prev < x && curr > x {
		return true
	}
	return false
}

// CrossDown returns true if the latest series values cross below x
func crossUnder(curr, prev, x float64) bool {
	if prev > x && curr < x {
		return true
	}
	return false
}

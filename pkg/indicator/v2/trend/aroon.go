package trend

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

//  Aroon Indicator

// The [Aroon](https://pkg.go.dev/github.com/cinar/indicator#Aroon) function calculates
// a technical indicator that is used to identify trend changes in the price of a stock,
// as well as the strength of that trend. It consists of two lines, Aroon Up, and Aroon Down.
// The Aroon Up line measures the strength of the uptrend, and the Aroon Down measures
// the strength of the downtrend. When Aroon Up is above Aroon Down, it indicates bullish price,
// and when Aroon Down is above Aroon Up, it indicates bearish price.

// ```
//
//	Aroon Up = ((25 - Period Since Last 25 Period High) / 25) * 100
//	Aroon Down = ((25 - Period Since Last 25 Period Low) / 25) * 100
//
// ```
type AroonStream struct {
	*types.Float64Series

	window, lowIndex int
	direction        float64
}

// AroonUpIndicator returns a derivative indicator that will return a value based on
// the number of ticks since the highest price in the window
// https://www.investopedia.com/terms/a/aroon.asp
//
// Note: this indicator should be constructed with a either a HighPriceIndicator or a derivative thereof
func AroonUpIndicator(source types.Float64Source, window int) *AroonStream {
	s := &AroonStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		lowIndex:      -1,
		direction:     -1.0,
	}
	s.Bind(source, s)
	return s
}

// AroonDownIndicator returns a derivative indicator that will return a value based on
// the number of ticks since the lowest price in the window
// https://www.investopedia.com/terms/a/aroon.asp
//
// Note: this indicator should be constructed with a either a LowPriceIndicator or a derivative thereof
func AroonDownIndicator(source types.Float64Source, window int) *AroonStream {
	s := &AroonStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		lowIndex:      -1,
		direction:     1.0,
	}
	s.Bind(source, s)
	return s
}

func (s *AroonStream) Calculate(v float64) float64 {
	if s.Length() < s.window-1 {
		return 0
	}

	var (
		index  = s.Length() - 1
		pSince = index - s.findLowIndex(index)
		aroon  = float64(s.window-pSince) / float64(s.window*100)
	)

	return aroon
}

func (s *AroonStream) findLowIndex(index int) int {
	if s.lowIndex < 1 || s.lowIndex < index-s.window {
		lv := math.MaxFloat64
		lowIndex := -1
		for i := (index + 1) - s.window; i <= index; i++ {
			value := s.Last(i) * s.direction
			if value < lv {
				lv = value
				lowIndex = i
			}
		}

		return lowIndex
	}

	v1 := s.Last(0) * s.direction
	v2 := s.Last(s.lowIndex) * s.direction

	if v1 < v2 {
		return index
	}

	return s.lowIndex
}

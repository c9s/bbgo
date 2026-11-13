package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// ALMAStream implements the Arnaud Legoux Moving Average
//
// Refer: Arnaud Legoux Moving Average
// Refer: https://capital.com/arnaud-legoux-moving-average
// Also check https://github.com/DaveSkender/Stock.Indicators/blob/main/src/a-d/Alma/Alma.cs
//
// The Arnaud Legoux Moving Average (ALMA) is a technical analysis indicator that is used to smooth price data and reduce the lag associated
// with traditional moving averages. It was developed by Arnaud Legoux and is based on the weighted moving average, with the weighting factors
// determined using a Gaussian function. The ALMA is calculated by taking the weighted moving average of the input data using weighting factors
// that are based on the standard deviation of the data and the specified length of the moving average. This resulting average is then plotted
// on the price chart as a line, which can be used to make predictions about future price movements. The ALMA is typically more responsive to
// changes in the underlying data than a simple moving average, but may be less reliable in trending markets.
//
// No value is pushed (or emitted) until `window` input values have been received.
type ALMAStream struct {
	*types.Float64Series

	window int
	offset float64
	sigma  float64
	weight []float64
	sum    float64
	input  []float64
}

// ALMA creates an ALMA stream.
//
// offset: Gaussian applied to the combo line. 1->ema, 0->sma (recommended: 0.5 ~ 0.9)
// sigma: the standard deviation applied to the combo line. This makes the combo line sharper (recommended: 5 ~ 6)
func ALMA(source types.Float64Source, window int, offset, sigma float64) *ALMAStream {
	s := &ALMAStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		offset:        offset,
		sigma:         sigma,
		weight:        make([]float64, window),
		input:         make([]float64, 0, window+1),
	}

	m := offset * (float64(window) - 1.)
	sd := float64(window) / sigma
	for i := 0; i < window; i++ {
		diff := float64(i) - m
		wt := math.Exp(-diff * diff / 2. / sd / sd)
		s.sum += wt
		s.weight[i] = wt
	}

	if source != nil {
		s.Subscribe(source, func(v float64) {
			if y, ok := s.calculate(v); ok {
				s.PushAndEmit(y)
				s.Truncate()
			}
		})
	}

	return s
}

// calculate appends v into the input window and returns the weighted average
// once the window is filled.
func (s *ALMAStream) calculate(v float64) (float64, bool) {
	s.input = append(s.input, v)
	if len(s.input) < s.window {
		return 0, false
	}

	if len(s.input) > s.window {
		s.input = append(s.input[:0], s.input[len(s.input)-s.window:]...)
	}

	weightedSum := 0.0
	for i := 0; i < s.window; i++ {
		weightedSum += s.weight[s.window-i-1] * s.input[i]
	}

	return weightedSum / s.sum, true
}

func (s *ALMAStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

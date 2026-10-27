package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// FisherTransformStream implements the Fisher Transform
//
// The Fisher Transform is a technical analysis indicator that is used to identify potential turning points in the price of a security.
// It is based on the idea that prices tend to be normally distributed, with most price movements being small and relatively insignificant.
// The Fisher Transform converts this normal distribution into a symmetrical, Gaussian distribution, with a peak at zero and a range of -1 to +1.
// This transformation allows for more accurate identification of price extremes, which can be used to make predictions about potential trend reversals.
//
// For each input value, the value is normalized into [-1, 1] by the highest and lowest values of the
// last `window` inputs, and the output is 0.5 * ln((1 + x) / (1 - x)).
type FisherTransformStream struct {
	*types.Float64Series

	window int
	prices *types.Queue
}

func FisherTransform(source types.Float64Source, window int) *FisherTransformStream {
	s := &FisherTransformStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		prices:        types.NewQueue(window),
	}
	s.Bind(source, s)
	return s
}

func (s *FisherTransformStream) Calculate(v float64) float64 {
	s.prices.Update(v)
	highest := s.prices.Highest(s.window)
	lowest := s.prices.Lowest(s.window)
	if highest == lowest {
		return 0
	}

	x := 2*((v-lowest)/(highest-lowest)) - 1
	if x == 1 {
		x = 0.9999
	} else if x == -1 {
		x = -0.9999
	}

	return 0.5 * math.Log((1+x)/(1-x))
}

func (s *FisherTransformStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

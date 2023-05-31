package types

import "math"

type SigmoidResult struct {
	a Series
}

func (s *SigmoidResult) Last(i int) float64 {
	return 1. / (1. + math.Exp(-s.a.Last(i)))
}

func (s *SigmoidResult) Index(i int) float64 {
	return s.Last(i)
}

func (s *SigmoidResult) Length() int {
	return s.a.Length()
}

// Sigmoid returns the input values in range of -1 to 1
// along the sigmoid or s-shaped curve.
// Commonly used in machine learning while training neural networks
// as an activation function.
func Sigmoid(a Series) SeriesExtend {
	return NewSeries(&SigmoidResult{a})
}

package xmaker

import "math"

// DirectionDivergence computes (m, pSign, D2) given signals and weights.
// m     : weighted mean of signals
// pSign : total normalized weight aligned with mean direction and |s_i| >= tau
// D2    : 1 - pSign
func DirectionDivergence(signals []float64, weights []float64, tau float64, epsM float64) (m, pSign, D2 float64) {
	n := len(signals)
	if n == 0 {
		return 0, 0, 1
	}
	if len(weights) != n {
		weights = make([]float64, n)
		for i := range weights {
			weights[i] = 1
		}
	}
	weights = normalizeWeightsFloat(weights)

	// weighted mean
	for i := 0; i < n; i++ {
		m += weights[i] * signals[i]
	}
	if math.Abs(m) < epsM {
		return m, 0, 1
	}
	mSign := signf(m)
	p := 0.0
	for i := 0; i < n; i++ {
		if math.Abs(signals[i]) >= tau && signf(signals[i]) == mSign {
			p += weights[i]
		}
	}
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	return m, p, 1 - p
}

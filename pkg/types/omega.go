package types

// Determines the Omega ratio of a strategy
// See https://en.wikipedia.org/wiki/Omega_ratio for more details
//
// @param returns (Series): Series of profit/loss percentage every specific interval
// @param returnThresholds(float64): threshold for returns filtering
// @return Omega ratio for give return series and threshold
func Omega(returns Series, returnThresholds ...float64) float64 {
	threshold := 0.0
	if len(returnThresholds) > 0 {
		threshold = returnThresholds[0]
	} else {
		threshold = Mean(returns)
	}
	length := returns.Length()
	win := 0.0
	loss := 0.0
	for i := 0; i < length; i++ {
		out := threshold - returns.Last(i)
		if out > 0 {
			win += out
		} else {
			loss -= out
		}
	}
	return win / loss
}

package types

import (
	"math"
)

// Sharpe: Calcluates the sharpe ratio of access returns
//
// @param returns (Series): Series of profit/loss percentage every specific interval
// @param periods (int): Freq. of returns (252/365 for daily, 12 for monthy, 1 for annually)
// @param annualize (bool): return annualize sharpe?
// @param smart (bool): return smart sharpe ratio
func Sharpe(returns Series, periods int, annualize bool, smart bool) float64 {
	data := returns
	var divisor = Stdev(data, data.Length(), 1)
	if smart {
		divisor *= autocorrPenalty(returns)
	}
	if divisor == 0 {
		mean := Mean(data)
		if mean > 0 {
			return math.Inf(1)
		} else if mean < 0 {
			return math.Inf(-1)
		} else {
			return 0
		}
	}
	result := Mean(data) / divisor
	if annualize {
		return result * math.Sqrt(float64(periods))
	}
	return result
}

func avgReturnRate(returnRate float64, periods int) float64 {
	return math.Pow(1.+returnRate, 1./float64(periods)) - 1.
}

func autocorrPenalty(data Series) float64 {
	num := data.Length()
	coef := math.Abs(Correlation(data, Shift(data, 1), num-1))
	var sum = 0.
	for i := 1; i < num; i++ {
		sum += float64(num-i) / float64(num) * math.Pow(coef, float64(i))
	}
	return math.Sqrt(1. + 2.*sum)
}

package types

import (
	"math"
)

// Sharpe: Calcluates the sharpe ratio of access returns
//
// @param returns (Series): Series of profit/loss percentage every specific interval
// @param periods (int): Freq. of returns (252/365 for daily, 12 for monthy)
// @param annualize (bool): return annualize sharpe?
// @param smart (bool): return smart sharpe ratio
func Sharpe(returns Series, periods int, annualize bool, smart bool) float64 {
	data := returns
	num := data.Length()
	divisor := Stdev(data, data.Length(), 1)
	if smart {
		sum := 0.
		coef := math.Abs(Correlation(data, Shift(data, 1), num-1))
		for i := 1; i < num; i++ {
			sum += float64(num-i) / float64(num) * math.Pow(coef, float64(i))
		}
		divisor = divisor * math.Sqrt(1.+2.*sum)
	}
	result := Mean(data) / divisor
	if annualize {
		return result * math.Sqrt(float64(periods))
	}
	return result
}

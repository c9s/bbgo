package statistics

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// Sharpe: Calcluates the sharpe ratio of access returns
//
// @param periods (int): Freq. of returns (252/365 for daily, 12 for monthy)
// @param annualize (bool): return annualize sharpe?
// @param smart (bool): return smart sharpe ratio
func Sharpe(returns types.Series, periods int, annualize bool, smart bool) float64 {
	data := returns
	num := data.Length()
	if types.Lowest(data, num) >= 0 && types.Highest(data, num) > 1 {
		data = types.PercentageChange(returns)
	}
	divisor := types.Stdev(data, data.Length(), 1)
	if smart {
		sum := 0.
		coef := math.Abs(types.Correlation(data, types.Shift(data, 1), num-1))
		for i := 1; i < num; i++ {
			sum += float64(num-i) / float64(num) * math.Pow(coef, float64(i))
		}
		divisor = divisor * math.Sqrt(1.+2.*sum)
	}
	result := types.Mean(data) / divisor
	if annualize {
		return result * math.Sqrt(float64(periods))
	}
	return result
}

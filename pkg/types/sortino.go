package types

import (
	"math"
)

// Sortino: Calcluates the sotino ratio of access returns
//
//	ROI_excess   E[ROI] - ROI_risk_free
//
// sortino = ---------- = -----------------------
//
//	risk      sqrt(E[ROI_drawdown^2])
//
// @param returns (Series): Series of profit/loss percentage every specific interval
// @param riskFreeReturns (float): risk-free return rate of year
// @param periods (int): Freq. of returns (252/365 for daily, 12 for monthy, 1 for annually)
// @param annualize (bool): return annualize sortino?
// @param smart (bool): return smart sharpe ratio
func Sortino(returns Series, riskFreeReturns float64, periods int, annualize bool, smart bool) float64 {
	avgRiskFreeReturns := 0.
	excessReturn := Mean(returns)
	if riskFreeReturns > 0. && periods > 0 {
		avgRiskFreeReturns = avgReturnRate(riskFreeReturns, periods)
		excessReturn -= avgRiskFreeReturns
	}

	num := returns.Length()
	if num == 0 {
		return 0
	}
	var sum = 0.
	for i := 0; i < num; i++ {
		exRet := returns.Last(i) - avgRiskFreeReturns
		if exRet < 0 {
			sum += exRet * exRet
		}
	}
	var risk = math.Sqrt(sum / float64(num))
	if smart {
		risk *= autocorrPenalty(returns)
	}
	if risk == 0 {
		if excessReturn > 0 {
			return math.Inf(1)
		} else if excessReturn < 0 {
			return math.Inf(-1)
		} else {
			return 0
		}
	}
	result := excessReturn / risk
	if annualize {
		return result * math.Sqrt(float64(periods))
	}
	return result
}

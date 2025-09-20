package types

import (
	"math"

	"gonum.org/v1/gonum/stat"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const (
	// DailyToAnnualFactor is the factor to scale daily observations to annual.
	// Commonly defined as the number of public market trading days in a year.
	DailyToAnnualFactor = 252 // todo does this apply to crypto at all?
)

// AnnualHistoricVolatility is the historic volatility of the equity curve as annualized std dev.
func AnnualHistoricVolatility(data Series) float64 {
	var sd = Stdev(data, data.Length(), 1)
	return sd * math.Sqrt(DailyToAnnualFactor)
}

// CAGR is the Compound Annual Growth Rate of the equity curve.
func CAGR(initial, final float64, days int) float64 {
	var (
		growthRate = (final - initial) / initial
		x          = 1 + growthRate
		y          = 365.0 / float64(days)
	)
	return math.Pow(x, y) - 1
}

// measures of risk-adjusted return based on drawdown risk

// calmar ratio - discounts expected excess return of a portfolio by the
// worst expected maximum draw down for that portfolio
// CR = E(re)/MD1 = (E(r) - rf) / MD1
func CalmarRatio(cagr, maxDrawdown float64) float64 {
	return cagr / maxDrawdown
}

// Sterling ratio
// discounts the expected excess return of a portfolio by the average of the N worst
// expected maximum drawdowns for that portfolio
// CR = E(re) / (1/N)(sum MDi)
func SterlingRatio(cagr, avgDrawdown float64) float64 {
	return cagr / avgDrawdown
}

// Burke Ratio
// similar to sterling, but less sensitive to outliers
// discounts the expected excess return of a portfolio by the square root of the average
// of the N worst expected maximum drawdowns for that portfolio
// BR = E(re) / ((1/N)(sum MD^2))^0.5            ---> smoothing, can take roots, logs etc
func BurkeRatio(cagr, avgDrawdownSquared float64) float64 {
	return cagr / math.Sqrt(avgDrawdownSquared)
}

// KellyCriterion the famous method for trade sizing.
func KellyCriterion(profitFactor, winP fixedpoint.Value) fixedpoint.Value {
	return profitFactor.Mul(winP).Sub(fixedpoint.One.Sub(winP)).Div(profitFactor)
}

// PRR (Pessimistic Return Ratio) is the profit factor with a penalty for a lower number of roundturns.
func PRR(profit, loss, winningN, losingN fixedpoint.Value) fixedpoint.Value {
	var (
		winF  = 1 / math.Sqrt(1+winningN.Float64())
		loseF = 1 / math.Sqrt(1+losingN.Float64())
	)
	return fixedpoint.NewFromFloat((1 - winF) / (1 + loseF) * (1 + profit.Float64()) / (1 + loss.Float64()))
}

// StatN returns the statistically significant number of samples required based on the distribution of a series.
// From: https://www.elitetrader.com/et/threads/minimum-number-of-roundturns-required-for-backtesting-results-to-be-trusted.356588/page-2
func StatN(xs floats.Slice) (sn, se fixedpoint.Value) {
	var (
		sd     = Stdev(xs, xs.Length(), 1)
		m      = Mean(xs)
		statn  = math.Pow(4*(sd/m), 2)
		stdErr = stat.StdErr(sd, float64(xs.Length()))
	)
	return fixedpoint.NewFromFloat(statn), fixedpoint.NewFromFloat(stdErr)
}

// OptimalF is a function that returns the 'OptimalF' for a series of trade returns as defined by Ralph Vince.
// It is a method for sizing positions to maximize geometric return whilst accounting for biggest trading loss.
// See: https://www.investopedia.com/terms/o/optimalf.asp
// Param roundturns is the series of profits (-ve amount for losses) for each trade
func OptimalF(roundturns floats.Slice) fixedpoint.Value {
	var (
		maxTWR, optimalF float64
		maxLoss          = roundturns.Min()
	)
	for i := 1.0; i <= 100.0; i++ {
		twr := 1.0
		f := i / 100
		for j := range roundturns {
			if roundturns[j] == 0 {
				continue
			}
			hpr := 1 + f*(-roundturns[j]/maxLoss)
			twr *= hpr
		}
		if twr > maxTWR {
			maxTWR = twr
			optimalF = f
		}
	}

	return fixedpoint.NewFromFloat(optimalF)
}

// NN (Not Number) returns y if x is NaN or Inf.
func NN(x, y float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return y
	}
	return x
}

// NNZ (Not Number or Zero) returns y if x is NaN or Inf or Zero.
func NNZ(x, y float64) float64 {
	if NN(x, y) == y || x == 0 {
		return y
	}
	return x
}

// Compute the drawdown function associated to a portfolio equity curve,
// also called the portfolio underwater equity curve.
// Portfolio Optimization with Drawdown Constraints, Chekhlov et al., 2000
// http://papers.ssrn.com/sol3/papers.cfm?abstract_id=223323
func Drawdown(equityCurve floats.Slice) floats.Slice {
	// Initialize highWaterMark
	highWaterMark := math.Inf(-1)

	// Create ddVector with the same length as equityCurve
	ddVector := make([]float64, len(equityCurve))

	// Loop over all the values to compute the drawdown vector
	for i := 0; i < len(equityCurve); i++ {
		if equityCurve[i] > highWaterMark {
			highWaterMark = equityCurve[i]
		}

		ddVector[i] = (highWaterMark - equityCurve[i]) / highWaterMark
	}

	return ddVector
}

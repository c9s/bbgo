package xfundingv2

import "github.com/c9s/bbgo/pkg/fixedpoint"

const annualFundingHours = 24 * 365

// AnnualizedRate converts a per-period funding rate to annualized rate
func AnnualizedRate(fundingRate fixedpoint.Value, fundingIntervalHours int) fixedpoint.Value {
	numFundingPeriods := int64(annualFundingHours / fundingIntervalHours)
	return fundingRate.Mul(fixedpoint.NewFromInt(numFundingPeriods))
}

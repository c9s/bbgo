package binanceapi

import (
	"math"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type HourlyInterestRate struct {
	Asset                  string           `json:"asset"`
	NextHourlyInterestRate fixedpoint.Value `json:"nextHourlyInterestRate"`
}

func (r *HourlyInterestRate) GetAnnualizedInterestRate() fixedpoint.Value {
	rf := r.NextHourlyInterestRate.Float64()
	return fixedpoint.NewFromFloat(math.Pow(rf+1.0, 24*365) - 1.0)
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/next-hourly-interest-rate" -type GetMarginFutureHourlyInterestRateRequest -responseType []HourlyInterestRate
type GetMarginFutureHourlyInterestRateRequest struct {
	client requestgen.AuthenticatedAPIClient

	// assets: List of assets, separated by commas, up to 20
	assets string `param:"assets"`

	// isIsolated: for isolated margin or not, "TRUE", "FALSE"
	isIsolated string `param:"isIsolated"` // TRUE or FALSE
}

func (c *RestClient) NewGetMarginFutureHourlyInterestRateRequest() *GetMarginFutureHourlyInterestRateRequest {
	return &GetMarginFutureHourlyInterestRateRequest{client: c}
}

package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type HourlyInterestRate struct {
	Asset                  string           `json:"asset"`
	NextHourlyInterestRate fixedpoint.Value `json:"nextHourlyInterestRate"`
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/next-hourly-interest-rate" -type GetMarginFutureHourlyInterestRateRequest -responseType []HourlyInterestRate
type GetMarginFutureHourlyInterestRateRequest struct {
	client requestgen.AuthenticatedAPIClient

	// assets: List of assets, separated by commas, up to 20
	assets string `param:"assets"`

	// isolated: for isolated margin or not, "TRUE", "FALSE"
	isolated string `param:"isolated"` // TRUE or FALSE
}

func (c *RestClient) NewGetMarginFutureHourlyInterestRateRequest() *GetMarginFutureHourlyInterestRateRequest {
	return &GetMarginFutureHourlyInterestRateRequest{client: c}
}

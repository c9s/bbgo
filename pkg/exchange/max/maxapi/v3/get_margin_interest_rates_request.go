package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type MarginInterestRate struct {
	HourlyInterestRate     fixedpoint.Value `json:"hourly_interest_rate"`
	NextHourlyInterestRate fixedpoint.Value `json:"next_hourly_interest_rate"`
}

type MarginInterestRateMap map[string]MarginInterestRate

//go:generate GetRequest -url "/api/v3/wallet/m/interest_rates" -type GetMarginInterestRatesRequest -responseType .MarginInterestRateMap
type GetMarginInterestRatesRequest struct {
	client requestgen.APIClient
}

func (s *Client) NewGetMarginInterestRatesRequest() *GetMarginInterestRatesRequest {
	return &GetMarginInterestRatesRequest{client: s.Client}
}

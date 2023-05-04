package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"time"

	"github.com/c9s/requestgen"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type MarginService struct {
	Client *maxapi.RestClient
}

func (s *Client) NewGetMarginInterestRatesRequest() *GetMarginInterestRatesRequest {
	return &GetMarginInterestRatesRequest{client: s.Client}
}

func (s *Client) NewGetMarginBorrowingLimitsRequest() *GetMarginBorrowingLimitsRequest {
	return &GetMarginBorrowingLimitsRequest{client: s.Client}
}

func (s *Client) NewGetMarginInterestHistoryRequest(currency string) *GetMarginInterestHistoryRequest {
	return &GetMarginInterestHistoryRequest{client: s.Client, currency: currency}
}

type MarginInterestRate struct {
	HourlyInterestRate     fixedpoint.Value `json:"hourly_interest_rate"`
	NextHourlyInterestRate fixedpoint.Value `json:"next_hourly_interest_rate"`
}

type MarginInterestRateMap map[string]MarginInterestRate

//go:generate GetRequest -url "/api/v3/wallet/m/interest_rates" -type GetMarginInterestRatesRequest -responseType .MarginInterestRateMap
type GetMarginInterestRatesRequest struct {
	client requestgen.APIClient
}

type MarginBorrowingLimitMap map[string]fixedpoint.Value

//go:generate GetRequest -url "/api/v3/wallet/m/limits" -type GetMarginBorrowingLimitsRequest -responseType .MarginBorrowingLimitMap
type GetMarginBorrowingLimitsRequest struct {
	client requestgen.APIClient
}

type MarginInterestRecord struct {
	Currency     string                     `json:"currency"`
	Amount       fixedpoint.Value           `json:"amount"`
	InterestRate fixedpoint.Value           `json:"interest_rate"`
	CreatedAt    types.MillisecondTimestamp `json:"created_at"`
}

//go:generate GetRequest -url "/api/v3/wallet/m/interests/history/:currency" -type GetMarginInterestHistoryRequest -responseType []MarginInterestRecord
type GetMarginInterestHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency  string     `param:"currency,slug,required"`
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	limit     *int       `param:"limit"`
}

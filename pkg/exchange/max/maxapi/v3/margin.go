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

func (s *MarginService) NewGetMarginInterestRatesRequest() *GetMarginInterestRatesRequest {
	return &GetMarginInterestRatesRequest{client: s.Client}
}

func (s *MarginService) NewGetMarginBorrowingLimitsRequest() *GetMarginBorrowingLimitsRequest {
	return &GetMarginBorrowingLimitsRequest{client: s.Client}
}

func (s *MarginService) NewGetMarginInterestHistoryRequest(currency string) *GetMarginInterestHistoryRequest {
	return &GetMarginInterestHistoryRequest{client: s.Client, currency: currency}
}

func (s *MarginService) NewGetMarginLiquidationHistoryRequest() *GetMarginLiquidationHistoryRequest {
	return &GetMarginLiquidationHistoryRequest{client: s.Client}
}

func (s *MarginService) NewGetMarginLoanHistoryRequest() *GetMarginLoanHistoryRequest {
	return &GetMarginLoanHistoryRequest{client: s.Client}
}

func (s *MarginService) NewMarginRepayRequest() *MarginRepayRequest {
	return &MarginRepayRequest{client: s.Client}
}

func (s *MarginService) NewMarginLoanRequest() *MarginLoanRequest {
	return &MarginLoanRequest{client: s.Client}
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

type LiquidationRecord struct {
	SN              string                     `json:"sn"`
	AdRatio         fixedpoint.Value           `json:"ad_ratio"`
	ExpectedAdRatio fixedpoint.Value           `json:"expected_ad_ratio"`
	CreatedAt       types.MillisecondTimestamp `json:"created_at"`
	State           LiquidationState           `json:"state"`
}

type LiquidationState string

const (
	LiquidationStateProcessing LiquidationState = "processing"
	LiquidationStateDebt       LiquidationState = "debt"
	LiquidationStateLiquidated LiquidationState = "liquidated"
)

//go:generate GetRequest -url "/api/v3/wallet/m/liquidations" -type GetMarginLiquidationHistoryRequest -responseType []LiquidationRecord
type GetMarginLiquidationHistoryRequest struct {
	client    requestgen.AuthenticatedAPIClient
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	limit     *int       `param:"limit"`
}

type RepaymentRecord struct {
	SN        string                     `json:"sn"`
	Currency  string                     `json:"currency"`
	Amount    fixedpoint.Value           `json:"amount"`
	Principal fixedpoint.Value           `json:"principal"`
	Interest  fixedpoint.Value           `json:"interest"`
	CreatedAt types.MillisecondTimestamp `json:"created_at"`
	State     string                     `json:"state"`
}

//go:generate GetRequest -url "/api/v3/wallet/m/repayments/:currency" -type GetMarginRepaymentHistoryRequest -responseType []RepaymentRecord
type GetMarginRepaymentHistoryRequest struct {
	client   requestgen.AuthenticatedAPIClient
	currency string `param:"currency,slug,required"`

	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	limit     *int       `param:"limit"`
}

type LoanRecord struct {
	SN           string                     `json:"sn"`
	Currency     string                     `json:"currency"`
	Amount       fixedpoint.Value           `json:"amount"`
	State        string                     `json:"state"`
	CreatedAt    types.MillisecondTimestamp `json:"created_at"`
	InterestRate fixedpoint.Value           `json:"interest_rate"`
}

//go:generate GetRequest -url "/api/v3/wallet/m/loans/:currency" -type GetMarginLoanHistoryRequest -responseType []LoanRecord
type GetMarginLoanHistoryRequest struct {
	client   requestgen.AuthenticatedAPIClient
	currency string `param:"currency,slug,required"`

	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	limit     *int       `param:"limit"`
}

//go:generate PostRequest -url "/api/v3/wallet/m/loan/:currency" -type MarginLoanRequest -responseType .LoanRecord
type MarginLoanRequest struct {
	client   requestgen.AuthenticatedAPIClient
	currency string `param:"currency,slug,required"`
	amount   string `param:"amount"`
}

//go:generate PostRequest -url "/api/v3/wallet/m/repayment/:currency" -type MarginRepayRequest -responseType .RepaymentRecord
type MarginRepayRequest struct {
	client   requestgen.AuthenticatedAPIClient
	currency string `param:"currency,slug,required"`
	amount   string `param:"amount"`
}

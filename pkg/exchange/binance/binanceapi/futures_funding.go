package binanceapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type FutureFundingRate struct {
	Symbol               string           `json:"symbol"`
	MarkPrice            string           `json:"markPrice"`
	IndexPrice           string           `json:"indexPrice"`
	EstimatedSettlePrice string           `json:"estimatedSettlePrice"`
	LastFundingRate      fixedpoint.Value `json:"lastFundingRate"`
	InterestRate         fixedpoint.Value `json:"interestRate"`
	NextFundingTime      types.Time       `json:"nextFundingTime"`
	Time                 types.Time       `json:"time"`
}

//go:generate requestgen -method GET -url "/fapi/v1/premiumIndex" -type FuturesFundingRateRequest -responseType []FutureFundingRate
type FuturesFundingRateRequest struct {
	client requestgen.APIClient

	symbol *string `param:"symbol"`
}

func (c *FuturesRestClient) NewFuturesFundingRateRequest() *FuturesFundingRateRequest {
	return &FuturesFundingRateRequest{client: c}
}

type FutureFundingInfo struct {
	Symbol                   string           `json:"symbol"`
	AdjustedFundingRateCap   fixedpoint.Value `json:"adjustedFundingRateCap"`
	AdjustedFundingRateFloor fixedpoint.Value `json:"adjustedFundingRateFloor"`
	FundingIntervalHours     int              `json:"fundingIntervalHours"`
	Disclaimer               bool             `json:"disclaimer"`
	UpdateTime               int64            `json:"updateTime"`
}

//go:generate requestgen -method GET -url /fapi/v1/fundingInfo -type FuturesFundingInfoRequest -responseType []FutureFundingInfo
type FuturesFundingInfoRequest struct {
	client requestgen.APIClient
}

func (c *FuturesRestClient) NewFuturesFundingInfoRequest() *FuturesFundingInfoRequest {
	return &FuturesFundingInfoRequest{client: c}
}

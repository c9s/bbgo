package binanceapi

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type rawPremiumIndex struct {
	Symbol          string           `json:"symbol"`
	MarkPrice       fixedpoint.Value `json:"markPrice"`
	LastFundingRate fixedpoint.Value `json:"lastFundingRate"`
	NextFundingTime int64            `json:"nextFundingTime"`
	Time            int64            `json:"time"`
}

func (p *rawPremiumIndex) PremiumIndex() types.PremiumIndex {
	return types.PremiumIndex{
		Symbol:          p.Symbol,
		MarkPrice:       p.MarkPrice,
		LastFundingRate: p.LastFundingRate,
		NextFundingTime: time.Unix(0, p.NextFundingTime*int64(time.Millisecond)),
		Time:            time.Unix(0, p.Time*int64(time.Millisecond)),
	}
}

//go:generate requestgen -method GET -url "/fapi/v1/premiumIndex" -type FuturesPremiumIndexRequest -responseType rawPremiumIndex
type FuturesPremiumIndexRequest struct {
	client requestgen.APIClient

	symbol string `param:"symbol,required"`
}

//go:generate requestgen -method GET -url "/fapi/v1/premiumIndex" -type FuturesPremiumIndicesRequest -responseType []rawPremiumIndex
type FuturesPremiumIndicesRequest struct {
	client requestgen.APIClient
}

func (c *FuturesRestClient) NewFuturesPremiumIndexRequest(symbol string) *FuturesPremiumIndexRequest {
	return &FuturesPremiumIndexRequest{client: c, symbol: symbol}
}

func (c *FuturesRestClient) NewFuturesPremiumIndicesRequest() *FuturesPremiumIndicesRequest {
	return &FuturesPremiumIndicesRequest{client: c}
}

type FuturesFundingInfo struct {
	Symbol                   string           `json:"symbol"`
	AdjustedFundingRateCap   fixedpoint.Value `json:"adjustedFundingRateCap"`
	AdjustedFundingRateFloor fixedpoint.Value `json:"adjustedFundingRateFloor"`
	FundingIntervalHours     int              `json:"fundingIntervalHours"`
	Disclaimer               bool             `json:"disclaimer"`
	UpdateTime               int64            `json:"updateTime"`
}

//go:generate requestgen -method GET -url /fapi/v1/fundingInfo -type FuturesFundingInfoRequest -responseType []FuturesFundingInfo
type FuturesFundingInfoRequest struct {
	client requestgen.APIClient
}

func (c *FuturesRestClient) NewFuturesFundingInfoRequest() *FuturesFundingInfoRequest {
	return &FuturesFundingInfoRequest{client: c}
}

type rawFundingRate struct {
	Symbol       string           `json:"symbol"`
	FundingRate_ fixedpoint.Value `json:"fundingRate"`
	FundingTime  int64            `json:"fundingTime"`
	Time         int64            `json:"time"`
}

func (f *rawFundingRate) FundingRate() types.FundingRate {
	return types.FundingRate{
		Symbol:      f.Symbol,
		FundingRate: f.FundingRate_,
		FundingTime: time.Unix(0, f.FundingTime*int64(time.Millisecond)),
		Time:        time.Unix(0, f.Time*int64(time.Millisecond)),
	}
}

//go:generate requestgen -method GET -url /fapi/v1/fundingRate -type FuturesFundingRateHistoryRequest -responseType []rawFundingRate
type FuturesFundingRateHistoryRequest struct {
	client requestgen.APIClient

	symbol   *string    `param:"symbol"`
	starTime *time.Time `param:"startTime"`
	endTime  *time.Time `param:"endTime"`
	limit    *int       `param:"limit"`
}

func (c *FuturesRestClient) NewFuturesFundingRateHistoryRequest() *FuturesFundingRateHistoryRequest {
	return &FuturesFundingRateHistoryRequest{client: c}
}

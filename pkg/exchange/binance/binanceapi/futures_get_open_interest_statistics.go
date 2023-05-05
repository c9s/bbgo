package binanceapi

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type FuturesOpenInterestStatistics struct {
	Symbol               string                     `json:"symbol"`
	SumOpenInterest      fixedpoint.Value           `json:"sumOpenInterest"`
	SumOpenInterestValue fixedpoint.Value           `json:"sumOpenInterestValue"`
	Timestamp            types.MillisecondTimestamp `json:"timestamp"`
}

//go:generate requestgen -method GET -url "/futures/data/openInterestHist" -type FuturesGetOpenInterestStatisticsRequest -responseType []FuturesOpenInterestStatistics
type FuturesGetOpenInterestStatisticsRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string         `param:"symbol,required"`
	period types.Interval `param:"period,required"`

	limit     *uint64    `param:"limit"`
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
}

func (c *FuturesRestClient) NewFuturesGetOpenInterestStatisticsRequest() *FuturesGetOpenInterestStatisticsRequest {
	return &FuturesGetOpenInterestStatisticsRequest{client: c}
}

package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

//go:generate requestgen -method GET -url "/fapi/v1/indexPriceKlines" -type FuturesIndexPriceKlinesRequest -responseType []FuturesKLine
type FuturesIndexPriceKlinesRequest struct {
	client requestgen.APIClient

	pair     string         `param:"pair,required"`
	interval types.Interval `param:"interval,required"`

	limit     *uint64    `param:"limit"`
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
}

func (c *FuturesRestClient) NewFuturesIndexPriceKlinesRequest() *FuturesIndexPriceKlinesRequest {
	return &FuturesIndexPriceKlinesRequest{client: c}
}

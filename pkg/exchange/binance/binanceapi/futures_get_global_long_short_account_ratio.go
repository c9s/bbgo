package binanceapi

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type FuturesGlobalLongShortAccountRatio struct {
	Symbol         string                     `json:"symbol"`
	LongShortRatio fixedpoint.Value           `json:"longShortRatio"`
	LongAccount    fixedpoint.Value           `json:"longAccount"`
	ShortAccount   fixedpoint.Value           `json:"shortAccount"`
	Timestamp      types.MillisecondTimestamp `json:"timestamp"`
}

//go:generate requestgen -method GET -url "/futures/data/globalLongShortAccountRatio" -type FuturesGlobalLongShortAccountRatioRequest -responseType []FuturesGlobalLongShortAccountRatio
type FuturesGlobalLongShortAccountRatioRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string         `param:"symbol"`
	period types.Interval `param:"period"`

	limit     *uint64    `param:"limit"`
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
}

func (c *FuturesRestClient) NewFuturesGlobalLongShortAccountRatioRequest() *FuturesGlobalLongShortAccountRatioRequest {
	return &FuturesGlobalLongShortAccountRatioRequest{client: c}
}

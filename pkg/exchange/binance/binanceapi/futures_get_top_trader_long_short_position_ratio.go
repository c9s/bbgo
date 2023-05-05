package binanceapi

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type FuturesTopTraderLongShortPositionRatio struct {
	Symbol         string                     `json:"symbol"`
	LongShortRatio fixedpoint.Value           `json:"longShortRatio"`
	LongAccount    fixedpoint.Value           `json:"longAccount"`
	ShortAccount   fixedpoint.Value           `json:"shortAccount"`
	Timestamp      types.MillisecondTimestamp `json:"timestamp"`
}

//go:generate requestgen -method GET -url "/futures/data/topLongShortPositionRatio" -type FuturesTopTraderLongShortPositionRatioRequest -responseType []FuturesTopTraderLongShortPositionRatio
type FuturesTopTraderLongShortPositionRatioRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string         `param:"symbol"`
	period types.Interval `param:"period"`

	limit     *uint64    `param:"limit"`
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
}

func (c *FuturesRestClient) NewFuturesTopTraderLongShortPositionRatioRequest() *FuturesTopTraderLongShortPositionRatioRequest {
	return &FuturesTopTraderLongShortPositionRatioRequest{client: c}
}

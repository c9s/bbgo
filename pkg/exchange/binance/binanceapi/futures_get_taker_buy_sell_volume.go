package binanceapi

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type FuturesTakerBuySellVolume struct {
	BuySellRatio fixedpoint.Value           `json:"buySellRatio"`
	BuyVol       fixedpoint.Value           `json:"buyVol"`
	SellVol      fixedpoint.Value           `json:"sellVol"`
	Timestamp    types.MillisecondTimestamp `json:"timestamp"`
}

//go:generate requestgen -method GET -url "/futures/data/takerlongshortRatio" -type FuturesTakerBuySellVolumeRequest -responseType []FuturesTakerBuySellVolume
type FuturesTakerBuySellVolumeRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string         `param:"symbol"`
	period types.Interval `param:"period"`

	limit     *uint64    `param:"limit"`
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
}

func (c *FuturesRestClient) NewFuturesTakerBuySellVolumeRequest() *FuturesTakerBuySellVolumeRequest {
	return &FuturesTakerBuySellVolumeRequest{client: c}
}

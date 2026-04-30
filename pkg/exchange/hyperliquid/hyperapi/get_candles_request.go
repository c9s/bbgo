package hyperapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type KLine struct {
	OpenPrice    fixedpoint.Value           `json:"o"`
	HighestPrice fixedpoint.Value           `json:"h"`
	LowestPrice  fixedpoint.Value           `json:"l"`
	ClosePrice   fixedpoint.Value           `json:"c"`
	Volume       fixedpoint.Value           `json:"v"`
	Interval     string                     `json:"i"`
	Symbol       string                     `json:"s"`
	Trades       uint64                     `json:"n"`
	StartTime    types.MillisecondTimestamp `json:"t"`
	EndTime      types.MillisecondTimestamp `json:"T"`
}

type CandleRequest struct {
	Coin      string `json:"coin"`
	Interval  string `json:"interval"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime,omitempty"`
}

//go:generate requestgen -method POST -url "/info" -type GetCandlesRequest -responseType []KLine
type GetCandlesRequest struct {
	client requestgen.APIClient

	metaType ReqTypeInfo `param:"type" default:"candleSnapshot" validValues:"candleSnapshot"`

	candleRequest CandleRequest `param:"req,required"`
}

func (c *Client) NewGetCandlesRequest() *GetCandlesRequest {
	return &GetCandlesRequest{
		client:   c,
		metaType: ReqCandleSnapshot,
	}
}

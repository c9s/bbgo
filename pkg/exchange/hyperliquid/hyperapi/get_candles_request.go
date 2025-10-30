package hyperapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Response.Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Response.Data

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
	StartTime    types.MillisecondTimestamp `json:"t"`
	EndTime      types.MillisecondTimestamp `json:"T"`
}

type CandleRequest struct {
	Coin      string         `json:"coin"`
	Interval  types.Interval `json:"interval"`
	StartTime int64          `json:"startTime"`
	EndTime   int64          `json:"endTime"`
}

//go:generate PostRequest -url "/info" -type GetCandlesRequest -responseDataType []KLine
type GetCandlesRequest struct {
	client requestgen.APIClient

	metaType ReqTypeInfo `param:"type" default:"candleSnapshot" validValues:"candleSnapshot"`

	CandleRequest CandleRequest `param:"req,required"`
}

func (c *Client) NewGetCandlesRequest() *GetCandlesRequest {
	return &GetCandlesRequest{
		client:   c,
		metaType: ReqCandleSnapshot,
	}
}

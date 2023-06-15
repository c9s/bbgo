package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Candle struct {
	Open     fixedpoint.Value           `json:"open"`
	High     fixedpoint.Value           `json:"high"`
	Low      fixedpoint.Value           `json:"low"`
	Close    fixedpoint.Value           `json:"close"`
	QuoteVol fixedpoint.Value           `json:"quoteVol"`
	BaseVol  fixedpoint.Value           `json:"baseVol"`
	UsdtVol  fixedpoint.Value           `json:"usdtVol"`
	Ts       types.MillisecondTimestamp `json:"ts"`
}

//go:generate GetRequest -url "/api/spot/v1/market/candles" -type GetCandlesRequest -responseDataType []Candle
type GetCandlesRequest struct {
	client requestgen.APIClient
}

func (c *RestClient) NewGetCandlesRequest() *GetCandlesRequest {
	return &GetCandlesRequest{client: c}
}

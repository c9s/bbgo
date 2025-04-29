package coinbase

import (
	"errors"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

// an array of [ time, low, high, open, close, volume ]
// `time` is the timestamp of the start time of the bucket, which is the seconds since epoch.
type RawCandle []fixedpoint.Value

type Candle struct {
	Time   types.Time       `json:"time"`
	Low    fixedpoint.Value `json:"low"`
	High   fixedpoint.Value `json:"high"`
	Open   fixedpoint.Value `json:"open"`
	Close  fixedpoint.Value `json:"close"`
	Volume fixedpoint.Value `json:"volume"`
}

type GetCandlesResponse []RawCandle

func (rc *RawCandle) Candle() (*Candle, error) {
	if rc == nil || len(*rc) != 6 {
		return nil, errors.New("invalid raw candle")
	}
	values := *rc
	return &Candle{
		Time:   types.Time(time.Unix(values[0].Int64(), 0)),
		Low:    values[1],
		High:   values[2],
		Open:   values[3],
		Close:  values[4],
		Volume: values[5],
	}, nil
}

// https://docs.cdp.coinbase.com/exchange/reference/exchangerestapi_getproductcandles
//
//go:generate requestgen -method GET -url /products/:product_id/candles -rateLimiter 1+20/2s -type GetCandlesRequest -responseType .GetCandlesResponse
type GetCandlesRequest struct {
	client requestgen.AuthenticatedAPIClient

	productID   string     `param:"product_id,slug,required"`
	granularity *string    `param:"granularity" validValues:"60,300,900,3600,21600,86400"`
	start       *time.Time `param:"start,seconds"`
	end         *time.Time `param:"end,seconds"`
}

func (client *RestAPIClient) NewGetCandlesRequest() *GetCandlesRequest {
	req := GetCandlesRequest{
		client: client,
	}
	return &req
}

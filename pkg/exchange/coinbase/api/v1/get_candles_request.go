package coinbase

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/requestgen"
)

type Candle struct {
	Time   time.Time        `json:"time"`
	Low    fixedpoint.Value `json:"low"`
	High   fixedpoint.Value `json:"high"`
	Open   fixedpoint.Value `json:"open"`
	Close  fixedpoint.Value `json:"close"`
	Volume fixedpoint.Value `json:"volume"`
}

type GetCandlesResponse []Candle

// https://docs.cdp.coinbase.com/exchange/reference/exchangerestapi_getproductcandles
//
//go:generate requestgen -method GET -url /products/:product_id/candles -type GetCandlesRequest -responseType .GetCandlesResponse
type GetCandlesRequest struct {
	client requestgen.AuthenticatedAPIClient

	productID   string  `param:"product_id,required"`
	granularity *string `param:"granularity" validValues:"60,300,900,3600,21600,86400"`
	start       *string `param:"start"`
	end         *string `param:"end"`
}

func (client *RestAPIClient) NewGetCandlesRequest(
	productID string,
	granularity, start, end *string,
) *GetCandlesRequest {
	req := GetCandlesRequest{
		client: client,
		start:  start,
		end:    end,
	}
	if granularity == nil {
		req.Granularity("60")
	}
	return &req
}

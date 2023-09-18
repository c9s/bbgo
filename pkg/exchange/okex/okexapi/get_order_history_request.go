package okexapi

import (
	"time"

	"github.com/c9s/requestgen"
)

//go:generate GetRequest -url "/api/v5/trade/orders-history-archive" -type GetOrderHistoryRequest -responseDataType .APIResponse
type GetOrderHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentType InstrumentType `param:"instType,query"`
	instrumentID   *string        `param:"instId,query"`
	orderType      *OrderType     `param:"ordType,query"`
	// underlying and instrumentFamil Applicable to FUTURES/SWAP/OPTION
	underlying       *string `param:"uly,query"`
	instrumentFamily *string `param:"instFamily,query"`

	state     *OrderState `param:"state,query"`
	after     *string     `param:"after,query"`
	before    *string     `param:"before,query"`
	startTime *time.Time  `param:"begin,query,milliseconds"`

	// endTime for each request, startTime and endTime can be any interval, but should be in last 3 months
	endTime *time.Time `param:"end,query,milliseconds"`

	// limit for data size per page. Default: 100
	limit *uint64 `param:"limit,query"`
}

type OrderList []OrderDetails

// NewGetOrderHistoriesRequest is descending order by createdTime
func (c *RestClient) NewGetOrderHistoryRequest() *GetOrderHistoryRequest {
	return &GetOrderHistoryRequest{
		client:         c,
		instrumentType: InstrumentTypeSpot,
	}
}

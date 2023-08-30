package okexapi

import (
	"time"

	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

//go:generate GetRequest -url "/api/v5/trade/orders-history-archive" -type GetOrderHistoriesRequest -responseDataType .APIResponse
type GetOrderHistoriesRequest struct {
	client requestgen.AuthenticatedAPIClient

	InstrumentType InstrumentType `param:"instType,query"`
	InstrumentID   *string        `param:"instId,query"`
	OrderType      *OrderType     `param:"ordType,query"`
	// Underlying and InstrumentFamil Applicable to FUTURES/SWAP/OPTION
	Underlying       *string `param:"uly,query"`
	InstrumentFamily *string `param:"instFamily,query"`

	State     *OrderState `param:"state,query"`
	category  *Category   `param:"category,query"`
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
func (c *RestClient) NewGetOrderHistoriesRequest() *GetOrderHistoriesRequest {
	return &GetOrderHistoriesRequest{
		client:         c,
		InstrumentType: InstrumentTypeSpot,
	}
}

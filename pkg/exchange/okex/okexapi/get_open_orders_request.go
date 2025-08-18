package okexapi

import (
	"time"

	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type OpenOrder struct {
	OrderDetail
	QuickMgnType string `json:"quickMgnType"`
}

//go:generate GetRequest -url "/api/v5/trade/orders-pending" -type GetOpenOrdersRequest -responseDataType []OpenOrder
type GetOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentType InstrumentType `param:"instType,query"`
	instrumentID   *string        `param:"instId,query"`
	orderType      *OrderType     `param:"ordType,query"`
	state          *OrderState    `param:"state,query"`
	category       *string        `param:"category,query"`
	// Pagination of data to return records earlier than the requested ordId
	after *string `param:"after,query"`
	// Pagination of data to return records newer than the requested ordId
	before *string `param:"before,query"`
	// Filter with a begin timestamp. Unix timestamp format in milliseconds, e.g. 1597026383085
	begin *time.Time `param:"begin,query,timestamp"`

	// Filter with an end timestamp. Unix timestamp format in milliseconds, e.g. 1597026383085
	end   *time.Time `param:"end,query,timestamp"`
	limit *string    `param:"limit,query"`
}

func (c *RestClient) NewGetOpenOrdersRequest() *GetOpenOrdersRequest {
	return &GetOpenOrdersRequest{
		client:         c,
		instrumentType: InstrumentTypeSpot,
	}
}

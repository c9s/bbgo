package ftxapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result
//go:generate -command DeleteRequest requestgen -method DELETE -responseType .APIResponse -responseDataField Result

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Order struct {
	CreatedAt     time.Time        `json:"createdAt"`
	Future        string           `json:"future"`
	Id            int              `json:"id"`
	Market        string           `json:"market"`
	Price         fixedpoint.Value `json:"price"`
	AvgFillPrice  fixedpoint.Value `json:"avgFillPrice"`
	Size          fixedpoint.Value `json:"size"`
	RemainingSize fixedpoint.Value `json:"remainingSize"`
	FilledSize    fixedpoint.Value `json:"filledSize"`
	Side          string           `json:"side"`
	Status        string           `json:"status"`
	Type          string           `json:"type"`
	ReduceOnly    bool             `json:"reduceOnly"`
	Ioc           bool             `json:"ioc"`
	PostOnly      bool             `json:"postOnly"`
	ClientId      *string          `json:"clientId"`
}

//go:generate GetRequest -url "/api/orders" -type GetOpenOrdersRequest -responseDataType []Order
type GetOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient
	market string `param:"market,query"`
}

func (c *RestClient) NewGetOpenOrdersRequest(market string) *GetOpenOrdersRequest {
	return &GetOpenOrdersRequest{
		client: c,
		market: market,
	}
}

//go:generate GetRequest -url "/api/orders/history" -type GetOrderHistoryRequest -responseDataType []Order
type GetOrderHistoryRequest struct {
	client    requestgen.AuthenticatedAPIClient

	market    string     `param:"market,query"`

	startTime *time.Time `param:"start_time,seconds,query"`
	endTime   *time.Time `param:"end_time,seconds,query"`
}

func (c *RestClient) NewGetOrderHistoryRequest(market string) *GetOrderHistoryRequest {
	return &GetOrderHistoryRequest{
		client: c,
		market: market,
	}
}

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
	Id            int64            `json:"id"`
	Market        string           `json:"market"`
	Price         fixedpoint.Value `json:"price"`
	AvgFillPrice  fixedpoint.Value `json:"avgFillPrice"`
	Size          fixedpoint.Value `json:"size"`
	RemainingSize fixedpoint.Value `json:"remainingSize"`
	FilledSize    fixedpoint.Value `json:"filledSize"`
	Side          Side             `json:"side"`
	Status        OrderStatus      `json:"status"`
	Type          OrderType        `json:"type"`
	ReduceOnly    bool             `json:"reduceOnly"`
	Ioc           bool             `json:"ioc"`
	PostOnly      bool             `json:"postOnly"`
	ClientId      string           `json:"clientId"`
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
	client requestgen.AuthenticatedAPIClient

	market string `param:"market,query"`

	startTime *time.Time `param:"start_time,seconds,query"`
	endTime   *time.Time `param:"end_time,seconds,query"`
}

func (c *RestClient) NewGetOrderHistoryRequest(market string) *GetOrderHistoryRequest {
	return &GetOrderHistoryRequest{
		client: c,
		market: market,
	}
}

//go:generate PostRequest -url "/api/orders" -type PlaceOrderRequest -responseDataType .Order
type PlaceOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	market    string           `param:"market,required"`
	side      Side             `param:"side,required"`
	price     fixedpoint.Value `param:"price"`
	size      fixedpoint.Value `param:"size"`
	orderType OrderType        `param:"type"`
	ioc       *bool            `param:"ioc"`
	postOnly  *bool            `param:"postOnly"`
	clientID  *string          `param:"clientId,optional"`
}

func (c *RestClient) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{
		client: c,
	}
}

//go:generate requestgen -method DELETE -url "/api/orders/:orderID" -type CancelOrderRequest -responseType .APIResponse
type CancelOrderRequest struct {
	client  requestgen.AuthenticatedAPIClient
	orderID string `param:"orderID,required,slug"`
}

func (c *RestClient) NewCancelOrderRequest(orderID string) *CancelOrderRequest {
	return &CancelOrderRequest{
		client:  c,
		orderID: orderID,
	}
}

//go:generate requestgen -method DELETE -url "/api/orders" -type CancelAllOrderRequest -responseType .APIResponse
type CancelAllOrderRequest struct {
	client requestgen.AuthenticatedAPIClient
	market *string `param:"market"`
}

func (c *RestClient) NewCancelAllOrderRequest() *CancelAllOrderRequest {
	return &CancelAllOrderRequest{
		client: c,
	}
}

//go:generate requestgen -method DELETE -url "/api/orders/by_client_id/:clientOrderId" -type CancelOrderByClientOrderIdRequest -responseType .APIResponse
type CancelOrderByClientOrderIdRequest struct {
	client        requestgen.AuthenticatedAPIClient
	clientOrderId string `param:"clientOrderId,required,slug"`
}

func (c *RestClient) NewCancelOrderByClientOrderIdRequest(clientOrderId string) *CancelOrderByClientOrderIdRequest {
	return &CancelOrderByClientOrderIdRequest{
		client:        c,
		clientOrderId: clientOrderId,
	}
}

type Fill struct {
	// Id is fill ID
	Id            uint64           `json:"id"`
	Future        string           `json:"future"`
	Liquidity     Liquidity        `json:"liquidity"`
	Market        string           `json:"market"`
	BaseCurrency  string           `json:"baseCurrency"`
	QuoteCurrency string           `json:"quoteCurrency"`
	OrderId       uint64           `json:"orderId"`
	TradeId       uint64           `json:"tradeId"`
	Price         fixedpoint.Value `json:"price"`
	Side          Side             `json:"side"`
	Size          fixedpoint.Value `json:"size"`
	Time          time.Time        `json:"time"`
	Type          string           `json:"type"` // always = "order"
	Fee           fixedpoint.Value `json:"fee"`
	FeeCurrency   string           `json:"feeCurrency"`
	FeeRate       fixedpoint.Value `json:"feeRate"`
}

//go:generate GetRequest -url "/api/fills" -type GetFillsRequest -responseDataType []Fill
type GetFillsRequest struct {
	client requestgen.AuthenticatedAPIClient

	market    *string    `param:"market,query"`
	startTime *time.Time `param:"start_time,seconds,query"`
	endTime   *time.Time `param:"end_time,seconds,query"`
	orderID   *int       `param:"orderId,query"`

	// order is the order of the returned records, asc or null
	order *string `param:"order,query"`
}

func (c *RestClient) NewGetFillsRequest() *GetFillsRequest {
	return &GetFillsRequest{
		client: c,
	}
}

//go:generate GetRequest -url "/api/orders/:orderId" -type GetOrderStatusRequest -responseDataType .Order
type GetOrderStatusRequest struct {
	client  requestgen.AuthenticatedAPIClient
	orderID uint64 `param:"orderId,slug"`
}

func (c *RestClient) NewGetOrderStatusRequest(orderID uint64) *GetOrderStatusRequest {
	return &GetOrderStatusRequest{
		client:  c,
		orderID: orderID,
	}
}

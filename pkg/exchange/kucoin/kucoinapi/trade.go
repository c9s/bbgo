package kucoinapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data
//go:generate -command DeleteRequest requestgen -method DELETE -responseType .APIResponse -responseDataField Data

import (
	"context"
	"time"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type TradeService struct {
	client *RestClient
}

type OrderResponse struct {
	OrderID string `json:"orderId"`
}

func (c *TradeService) NewListHistoryOrdersRequest() *ListHistoryOrdersRequest {
	return &ListHistoryOrdersRequest{client: c.client}

}

func (c *TradeService) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{client: c.client}
}

func (c *TradeService) NewBatchPlaceOrderRequest() *BatchPlaceOrderRequest {
	return &BatchPlaceOrderRequest{client: c.client}
}

func (c *TradeService) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{client: c.client}
}

func (c *TradeService) NewCancelAllOrderRequest() *CancelAllOrderRequest {
	return &CancelAllOrderRequest{client: c.client}
}

func (c *TradeService) NewGetFillsRequest() *GetFillsRequest {
	return &GetFillsRequest{client: c.client}
}

//go:generate GetRequest -url /api/v1/fills -type GetFillsRequest -responseDataType .FillListPage
type GetFillsRequest struct {
	client requestgen.AuthenticatedAPIClient

	orderID *string `param:"orderId,query"`

	tradeType *string `param:"tradeType,query" default:"TRADE"`

	symbol *string `param:"symbol,query"`

	side *string `param:"side,query" validValues:"buy,sell"`

	orderType *string `param:"type,query" validValues:"limit,market,limit_stop,market_stop"`

	startAt *time.Time `param:"startAt,query,milliseconds"`

	endAt *time.Time `param:"endAt,query,milliseconds"`
}

type FillListPage struct {
	CurrentPage int    `json:"currentPage"`
	PageSize    int    `json:"pageSize"`
	TotalNumber int    `json:"totalNum"`
	TotalPage   int    `json:"totalPage"`
	Items       []Fill `json:"items"`
}

type Fill struct {
	Symbol         string                     `json:"symbol"`
	TradeId        string                     `json:"tradeId"`
	OrderId        string                     `json:"orderId"`
	CounterOrderId string                     `json:"counterOrderId"`
	Side           SideType                   `json:"side"`
	Liquidity      LiquidityType              `json:"liquidity"`
	ForceTaker     bool                       `json:"forceTaker"`
	Price          fixedpoint.Value           `json:"price"`
	Size           fixedpoint.Value           `json:"size"`
	Funds          fixedpoint.Value           `json:"funds"`
	Fee            fixedpoint.Value           `json:"fee"`
	FeeRate        fixedpoint.Value           `json:"feeRate"`
	FeeCurrency    string                     `json:"feeCurrency"`
	Stop           string                     `json:"stop"`
	Type           OrderType                  `json:"type"`
	CreatedAt      types.MillisecondTimestamp `json:"createdAt"`
	TradeType      TradeType                  `json:"tradeType"`
}

//go:generate GetRequest -url /api/v1/hist-orders -type ListHistoryOrdersRequest -responseDataType .HistoryOrderListPage
type ListHistoryOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol *string `param:"symbol"`

	startAt *time.Time `param:"startAt,milliseconds"`

	endAt *time.Time `param:"endAt,milliseconds"`
}

type HistoryOrder struct {
	Symbol    string `json:"symbol"`
	DealPrice string `json:"dealPrice"`
	DealValue string `json:"dealValue"`
	Amount    string `json:"amount"`
	Fee       string `json:"fee"`
	Side      string `json:"side"`
	CreatedAt int    `json:"createdAt"`
}

type HistoryOrderListPage struct {
	CurrentPage int            `json:"currentPage"`
	PageSize    int            `json:"pageSize"`
	TotalNum    int            `json:"totalNum"`
	TotalPage   int            `json:"totalPage"`
	Items       []HistoryOrder `json:"items"`
}

//go:generate GetRequest -url /api/v1/orders -type ListOrdersRequest -responseDataType .OrderListPage
type ListOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	status *string `param:"status" validValues:"active,done"`

	symbol *string `param:"symbol"`

	side *SideType `param:"side" validValues:"buy,sell"`

	orderType *OrderType `param:"type"`

	tradeType *TradeType `param:"tradeType" default:"TRADE"`

	startAt *time.Time `param:"startAt,milliseconds"`

	endAt *time.Time `param:"endAt,milliseconds"`
}

type Order struct {
	ID             string                     `json:"id"`
	Symbol         string                     `json:"symbol"`
	OperationType  string                     `json:"opType"`
	Type           string                     `json:"type"`
	Side           string                     `json:"side"`
	Price          fixedpoint.Value           `json:"price"`
	Size           fixedpoint.Value           `json:"size"`
	Funds          fixedpoint.Value           `json:"funds"`
	DealFunds      fixedpoint.Value           `json:"dealFunds"`
	DealSize       fixedpoint.Value           `json:"dealSize"`
	Fee            fixedpoint.Value           `json:"fee"`
	FeeCurrency    string                     `json:"feeCurrency"`
	StopType       string                     `json:"stop"`
	StopTriggerred bool                       `json:"stopTriggered"`
	StopPrice      fixedpoint.Value           `json:"stopPrice"`
	TimeInForce    TimeInForceType            `json:"timeInForce"`
	PostOnly       bool                       `json:"postOnly"`
	Hidden         bool                       `json:"hidden"`
	Iceberg        bool                       `json:"iceberg"`
	Channel        string                     `json:"channel"`
	ClientOrderID  string                     `json:"clientOid"`
	Remark         string                     `json:"remark"`
	IsActive       bool                       `json:"isActive"`
	CancelExist    bool                       `json:"cancelExist"`
	CreatedAt      types.MillisecondTimestamp `json:"createdAt"`
}

type OrderListPage struct {
	CurrentPage int     `json:"currentPage"`
	PageSize    int     `json:"pageSize"`
	TotalNumber int     `json:"totalNum"`
	TotalPage   int     `json:"totalPage"`
	Items       []Order `json:"items"`
}

func (c *TradeService) NewListOrdersRequest() *ListOrdersRequest {
	return &ListOrdersRequest{client: c.client}
}

//go:generate PostRequest -url /api/v1/orders -type PlaceOrderRequest -responseDataType .OrderResponse
type PlaceOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string `param:"clientOid,required" defaultValuer:"uuid()"`

	symbol string `param:"symbol,required"`

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string `param:"tag"`

	// "buy" or "sell"
	side SideType `param:"side"`

	orderType OrderType `param:"ordType"`

	// limit order parameters
	size string `param:"size,required"`

	price *string `param:"price"`

	timeInForce *TimeInForceType `param:"timeInForce,required"`

	postOnly *bool `param:"postOnly"`
}

type CancelOrderResponse struct {
	CancelledOrderIDs []string `json:"cancelledOrderIds,omitempty"`

	// used when using client order id for canceling order
	CancelledOrderId string `json:"cancelledOrderId,omitempty"`
	ClientOrderID    string `json:"clientOid,omitempty"`
}

//go:generate requestgen -type CancelOrderRequest
type CancelOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	orderID       *string `param:"orderID"`
	clientOrderID *string `param:"clientOrderID"`
}

func (r *CancelOrderRequest) Do(ctx context.Context) (*CancelOrderResponse, error) {
	if r.orderID == nil && r.clientOrderID == nil {
		return nil, errors.New("either orderID or clientOrderID is required for canceling order")
	}

	var refURL string

	if r.orderID != nil {
		refURL = "/api/v1/orders/" + *r.orderID
	} else if r.clientOrderID != nil {
		refURL = "/api/v1/order/client-order/" + *r.clientOrderID
	}

	req, err := r.client.NewAuthenticatedRequest(ctx, "DELETE", refURL, nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string               `json:"code"`
		Message string               `json:"msg"`
		Data    *CancelOrderResponse `json:"data"`
	}
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	if apiResponse.Data == nil {
		return nil, errors.New("api error: [" + apiResponse.Code + "] " + apiResponse.Message)
	}

	return apiResponse.Data, nil
}

//go:generate DeleteRequest -url /api/v1/orders -type CancelAllOrderRequest -responseDataType .CancelOrderResponse
type CancelAllOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol    *string `param:"symbol"`
	tradeType *string `param:"tradeType"`
}

// Request via this endpoint to place 5 orders at the same time.
// The order type must be a limit order of the same symbol.
// The interface currently only supports spot trading
type BatchPlaceOrderRequest struct {
	client *RestClient

	symbol string
	reqs   []*PlaceOrderRequest
}

func (r *BatchPlaceOrderRequest) Symbol(symbol string) *BatchPlaceOrderRequest {
	r.symbol = symbol
	return r
}

func (r *BatchPlaceOrderRequest) Add(reqs ...*PlaceOrderRequest) *BatchPlaceOrderRequest {
	r.reqs = append(r.reqs, reqs...)
	return r
}

func (r *BatchPlaceOrderRequest) Do(ctx context.Context) ([]OrderResponse, error) {
	var orderList []map[string]interface{}
	for _, req := range r.reqs {
		params, err := req.GetParameters()
		if err != nil {
			return nil, err
		}

		orderList = append(orderList, params)
	}

	var payload = map[string]interface{}{
		"symbol":    r.symbol,
		"orderList": orderList,
	}

	req, err := r.client.NewAuthenticatedRequest(ctx, "POST", "/api/v1/orders/multi", nil, payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string          `json:"code"`
		Message string          `json:"msg"`
		Data    []OrderResponse `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	if apiResponse.Data == nil {
		return nil, errors.New("api error: [" + apiResponse.Code + "] " + apiResponse.Message)
	}

	return apiResponse.Data, nil
}

package kucoinapi

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type TradeService struct {
	client *RestClient
}

type OrderResponse struct {
	OrderID string `json:"orderId"`
}

func (c *TradeService) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{
		client: c.client,
	}
}

func (c *TradeService) NewBatchPlaceOrderRequest() *BatchPlaceOrderRequest {
	return &BatchPlaceOrderRequest{
		client: c.client,
	}
}

func (c *TradeService) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{
		client: c.client,
	}
}

func (c *TradeService) NewCancelAllOrderRequest() *CancelAllOrderRequest {
	return &CancelAllOrderRequest{
		client: c.client,
	}
}

type ListOrdersRequest struct {
	client *RestClient

	status *string

	symbol *string

	side *SideType

	orderType *OrderType

	tradeType *TradeType

	startAt *time.Time

	endAt *time.Time
}

func (r *ListOrdersRequest) Status(status string) {
	r.status = &status
}

func (r *ListOrdersRequest) Symbol(symbol string) {
	r.symbol = &symbol
}

func (r *ListOrdersRequest) Side(side SideType) {
	r.side = &side
}

func (r *ListOrdersRequest) OrderType(orderType OrderType) {
	r.orderType = &orderType
}

func (r *ListOrdersRequest) StartAt(startAt time.Time) {
	r.startAt = &startAt
}

func (r *ListOrdersRequest) EndAt(endAt time.Time) {
	r.endAt = &endAt
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

func (r *ListOrdersRequest) Do(ctx context.Context) (*OrderListPage, error) {
	var params = url.Values{}

	if r.status != nil {
		params["status"] = []string{*r.status}
	}

	if r.symbol != nil {
		params["symbol"] = []string{*r.symbol}
	}

	if r.side != nil {
		params["side"] = []string{string(*r.side)}
	}

	if r.orderType != nil {
		params["type"] = []string{string(*r.orderType)}
	}

	if r.tradeType != nil {
		params["tradeType"] = []string{string(*r.tradeType)}
	} else {
		params["tradeType"] = []string{"TRADE"}
	}

	if r.startAt != nil {
		params["startAt"] = []string{strconv.FormatInt(r.startAt.UnixNano()/int64(time.Millisecond), 10)}
	}

	if r.endAt != nil {
		params["endAt"] = []string{strconv.FormatInt(r.endAt.UnixNano()/int64(time.Millisecond), 10)}
	}

	req, err := r.client.newAuthenticatedRequest("GET", "/api/v1/orders", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var orderResponse struct {
		Code    string         `json:"code"`
		Message string         `json:"msg"`
		Data    *OrderListPage `json:"data"`
	}

	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	if orderResponse.Data == nil {
		return nil, errors.New("api error: [" + orderResponse.Code + "] " + orderResponse.Message)
	}

	return orderResponse.Data, nil
}

func (c *TradeService) NewListOrdersRequest() *ListOrdersRequest {
	return &ListOrdersRequest{client: c.client}
}

type PlaceOrderRequest struct {
	client *RestClient

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string

	symbol string

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string

	// "buy" or "sell"
	side SideType

	ordType OrderType

	// limit order parameters
	size string

	price *string

	timeInForce *TimeInForceType
}

func (r *PlaceOrderRequest) Symbol(symbol string) *PlaceOrderRequest {
	r.symbol = symbol
	return r
}

func (r *PlaceOrderRequest) ClientOrderID(clientOrderID string) *PlaceOrderRequest {
	r.clientOrderID = &clientOrderID
	return r
}

func (r *PlaceOrderRequest) Side(side SideType) *PlaceOrderRequest {
	r.side = side
	return r
}

func (r *PlaceOrderRequest) Size(size string) *PlaceOrderRequest {
	r.size = size
	return r
}

func (r *PlaceOrderRequest) Price(price string) *PlaceOrderRequest {
	r.price = &price
	return r
}

func (r *PlaceOrderRequest) TimeInForce(timeInForce TimeInForceType) *PlaceOrderRequest {
	r.timeInForce = &timeInForce
	return r
}

func (r *PlaceOrderRequest) OrderType(orderType OrderType) *PlaceOrderRequest {
	r.ordType = orderType
	return r
}

func (r *PlaceOrderRequest) getParameters() (map[string]interface{}, error) {
	payload := map[string]interface{}{}

	payload["symbol"] = r.symbol

	if r.clientOrderID != nil {
		payload["clientOid"] = r.clientOrderID
	} else {
		payload["clientOid"] = uuid.New().String()
	}

	if len(r.side) == 0 {
		return nil, errors.New("order side is required")
	}

	payload["side"] = r.side
	payload["type"] = r.ordType
	payload["size"] = r.size

	if r.price != nil {
		payload["price"] = r.price
	}

	if r.timeInForce != nil {
		payload["timeInForce"] = r.timeInForce
	}

	return payload, nil
}

func (r *PlaceOrderRequest) Do(ctx context.Context) (*OrderResponse, error) {
	payload, err := r.getParameters()
	if err != nil {
		return nil, err
	}

	req, err := r.client.newAuthenticatedRequest("POST", "/api/v1/orders", nil, payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var orderResponse struct {
		Code    string         `json:"code"`
		Message string         `json:"msg"`
		Data    *OrderResponse `json:"data"`
	}

	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	if orderResponse.Data == nil {
		return nil, errors.New("api error: [" + orderResponse.Code + "] " + orderResponse.Message)
	}

	return orderResponse.Data, nil
}

type CancelOrderRequest struct {
	client *RestClient

	orderID       *string
	clientOrderID *string
}

func (r *CancelOrderRequest) OrderID(orderID string) *CancelOrderRequest {
	r.orderID = &orderID
	return r
}

func (r *CancelOrderRequest) ClientOrderID(clientOrderID string) *CancelOrderRequest {
	r.clientOrderID = &clientOrderID
	return r
}

type CancelOrderResponse struct {
	CancelledOrderIDs []string `json:"cancelledOrderIds,omitempty"`

	// used when using client order id for canceling order
	CancelledOrderId string `json:"cancelledOrderId,omitempty"`
	ClientOrderID    string `json:"clientOid,omitempty"`
}

func (r *CancelOrderRequest) Do(ctx context.Context) (*CancelOrderResponse, error) {
	if r.orderID == nil || r.clientOrderID == nil {
		return nil, errors.New("either orderID or clientOrderID is required for canceling order")
	}

	var refURL string

	if r.orderID != nil {
		refURL = "/api/v1/orders/" + *r.orderID
	} else if r.clientOrderID != nil {
		refURL = "/api/v1/order/client-order/" + *r.clientOrderID
	}

	req, err := r.client.newAuthenticatedRequest("DELETE", refURL, nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
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

type CancelAllOrderRequest struct {
	client *RestClient

	symbol *string

	// tradeType string
}

func (r *CancelAllOrderRequest) Symbol(symbol string) *CancelAllOrderRequest {
	r.symbol = &symbol
	return r
}

func (r *CancelAllOrderRequest) Do(ctx context.Context) (*CancelOrderResponse, error) {
	req, err := r.client.newAuthenticatedRequest("DELETE", "/api/v1/orders", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
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
		params, err := req.getParameters()
		if err != nil {
			return nil, err
		}

		orderList = append(orderList, params)
	}

	var payload = map[string]interface{}{
		"symbol":    r.symbol,
		"orderList": orderList,
	}

	req, err := r.client.newAuthenticatedRequest("POST", "/api/v1/orders/multi", nil, payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var orderResponse struct {
		Code    string          `json:"code"`
		Message string          `json:"msg"`
		Data    []OrderResponse `json:"data"`
	}
	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	return orderResponse.Data, nil
}

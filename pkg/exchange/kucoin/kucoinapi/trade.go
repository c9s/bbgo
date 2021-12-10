package kucoinapi

import (
	"context"
	"net/url"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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

func (c *TradeService) NewGetOrderDetailsRequest() *GetOrderDetailsRequest {
	return &GetOrderDetailsRequest{
		client: c.client,
	}
}

func (c *TradeService) NewGetPendingOrderRequest() *GetPendingOrderRequest {
	return &GetPendingOrderRequest{
		client: c.client,
	}
}

func (c *TradeService) NewGetTransactionDetailsRequest() *GetTransactionDetailsRequest {
	return &GetTransactionDetailsRequest{
		client: c.client,
	}
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

func (r *PlaceOrderRequest) Quantity(quantity string) *PlaceOrderRequest {
	r.size = quantity
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

func (r *PlaceOrderRequest) getParameters() map[string]interface{} {
	payload := map[string]interface{}{}

	payload["symbol"] = r.symbol

	if r.clientOrderID != nil {
		payload["clientOid"] = r.clientOrderID
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

	return payload
}

func (r *PlaceOrderRequest) Do(ctx context.Context) (*OrderResponse, error) {
	payload := r.getParameters()
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
	CancelledOrderId  string   `json:"cancelledOrderId,omitempty"`
	ClientOrderID     string   `json:"clientOid,omitempty"`
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

	var orderResponse struct {
		Code    string               `json:"code"`
		Message string               `json:"msg"`
		Data    *CancelOrderResponse `json:"data"`
	}
	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	return orderResponse.Data, nil
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

	var orderResponse struct {
		Code    string          `json:"code"`
		Message string          `json:"msg"`
		Data    *CancelOrderResponse `json:"data"`
	}

	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	return orderResponse.Data, nil
}

// Request via this endpoint to place 5 orders at the same time.
// The order type must be a limit order of the same symbol.
// The interface currently only supports spot trading
type BatchPlaceOrderRequest struct {
	client *RestClient

	symbol string
	reqs []*PlaceOrderRequest
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
		params := req.getParameters()
		orderList = append(orderList, params)
	}

	var payload = map[string]interface{}{
		"symbol": r.symbol,
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

type OrderDetails struct {
	InstrumentType string           `json:"instType"`
	InstrumentID   string           `json:"instId"`
	Tag            string           `json:"tag"`
	Price          fixedpoint.Value `json:"px"`
	Quantity       fixedpoint.Value `json:"sz"`

	OrderID       string    `json:"ordId"`
	ClientOrderID string    `json:"clOrdId"`
	OrderType     OrderType `json:"ordType"`
	Side          SideType  `json:"side"`

	// Accumulated fill quantity
	FilledQuantity fixedpoint.Value `json:"accFillSz"`

	FeeCurrency string           `json:"feeCcy"`
	Fee         fixedpoint.Value `json:"fee"`

	// trade related fields
	LastTradeID           string                     `json:"tradeId,omitempty"`
	LastFilledPrice       fixedpoint.Value           `json:"fillPx"`
	LastFilledQuantity    fixedpoint.Value           `json:"fillSz"`
	LastFilledTime        types.MillisecondTimestamp `json:"fillTime"`
	LastFilledFee         fixedpoint.Value           `json:"fillFee"`
	LastFilledFeeCurrency string                     `json:"fillFeeCcy"`

	// ExecutionType = liquidity (M = maker or T = taker)
	ExecutionType string `json:"execType"`

	// Average filled price. If none is filled, it will return 0.
	AveragePrice fixedpoint.Value `json:"avgPx"`

	// Currency = Margin currency
	// Only applicable to cross MARGIN orders in Single-currency margin.
	Currency string `json:"ccy"`

	// Leverage = from 0.01 to 125.
	// Only applicable to MARGIN/FUTURES/SWAP
	Leverage fixedpoint.Value `json:"lever"`

	RebateCurrency string           `json:"rebateCcy"`
	Rebate         fixedpoint.Value `json:"rebate"`

	PnL fixedpoint.Value `json:"pnl"`

	UpdateTime   types.MillisecondTimestamp `json:"uTime"`
	CreationTime types.MillisecondTimestamp `json:"cTime"`

	State OrderState `json:"state"`
}

type GetOrderDetailsRequest struct {
	client *RestClient

	instId  string
	ordId   *string
	clOrdId *string
}

func (r *GetOrderDetailsRequest) InstrumentID(instId string) *GetOrderDetailsRequest {
	r.instId = instId
	return r
}

func (r *GetOrderDetailsRequest) OrderID(orderID string) *GetOrderDetailsRequest {
	r.ordId = &orderID
	return r
}

func (r *GetOrderDetailsRequest) ClientOrderID(clientOrderID string) *GetOrderDetailsRequest {
	r.clOrdId = &clientOrderID
	return r
}

func (r *GetOrderDetailsRequest) QueryParameters() url.Values {
	var values = url.Values{}

	values.Add("instId", r.instId)

	if r.ordId != nil {
		values.Add("ordId", *r.ordId)
	} else if r.clOrdId != nil {
		values.Add("clOrdId", *r.clOrdId)
	}

	return values
}

func (r *GetOrderDetailsRequest) Do(ctx context.Context) (*OrderDetails, error) {
	params := r.QueryParameters()
	req, err := r.client.newAuthenticatedRequest("GET", "/api/v5/trade/order", params, nil)
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
		Data    []OrderDetails `json:"data"`
	}
	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	if len(orderResponse.Data) == 0 {
		return nil, errors.New("order create error")
	}

	return &orderResponse.Data[0], nil
}

type GetPendingOrderRequest struct {
	client *RestClient

	instId *string

	instType *InstrumentType

	orderTypes []string

	state *OrderState
}

func (r *GetPendingOrderRequest) InstrumentID(instId string) *GetPendingOrderRequest {
	r.instId = &instId
	return r
}

func (r *GetPendingOrderRequest) InstrumentType(instType InstrumentType) *GetPendingOrderRequest {
	r.instType = &instType
	return r
}

func (r *GetPendingOrderRequest) State(state OrderState) *GetPendingOrderRequest {
	r.state = &state
	return r
}

func (r *GetPendingOrderRequest) OrderTypes(orderTypes []string) *GetPendingOrderRequest {
	r.orderTypes = orderTypes
	return r
}

func (r *GetPendingOrderRequest) AddOrderTypes(orderTypes ...string) *GetPendingOrderRequest {
	r.orderTypes = append(r.orderTypes, orderTypes...)
	return r
}

func (r *GetPendingOrderRequest) Parameters() map[string]interface{} {
	var payload = map[string]interface{}{}

	if r.instId != nil {
		payload["instId"] = r.instId
	}

	if r.instType != nil {
		payload["instType"] = r.instType
	}

	if r.state != nil {
		payload["state"] = r.state
	}

	if len(r.orderTypes) > 0 {
		payload["ordType"] = strings.Join(r.orderTypes, ",")
	}

	return payload
}

func (r *GetPendingOrderRequest) Do(ctx context.Context) ([]OrderDetails, error) {
	payload := r.Parameters()
	req, err := r.client.newAuthenticatedRequest("GET", "/api/v5/trade/orders-pending", nil, payload)
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
		Data    []OrderDetails `json:"data"`
	}
	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	return orderResponse.Data, nil
}

type GetTransactionDetailsRequest struct {
	client *RestClient

	instType *InstrumentType

	instId *string

	ordId *string
}

func (r *GetTransactionDetailsRequest) InstrumentType(instType InstrumentType) *GetTransactionDetailsRequest {
	r.instType = &instType
	return r
}

func (r *GetTransactionDetailsRequest) InstrumentID(instId string) *GetTransactionDetailsRequest {
	r.instId = &instId
	return r
}

func (r *GetTransactionDetailsRequest) OrderID(orderID string) *GetTransactionDetailsRequest {
	r.ordId = &orderID
	return r
}

func (r *GetTransactionDetailsRequest) Parameters() map[string]interface{} {
	var payload = map[string]interface{}{}

	if r.instType != nil {
		payload["instType"] = r.instType
	}

	if r.instId != nil {
		payload["instId"] = r.instId
	}

	if r.ordId != nil {
		payload["ordId"] = r.ordId
	}

	return payload
}

func (r *GetTransactionDetailsRequest) Do(ctx context.Context) ([]OrderDetails, error) {
	payload := r.Parameters()
	req, err := r.client.newAuthenticatedRequest("GET", "/api/v5/trade/fills", nil, payload)
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
		Data    []OrderDetails `json:"data"`
	}
	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	return orderResponse.Data, nil
}

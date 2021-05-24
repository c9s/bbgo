package okexapi

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

type CancelOrderRequest struct {
	client *RestClient

	instId  string
	ordId   *string
	clOrdId *string
}

func (r *CancelOrderRequest) InstrumentID(instId string) *CancelOrderRequest {
	r.instId = instId
	return r
}

func (r *CancelOrderRequest) OrderID(orderID string) *CancelOrderRequest {
	r.ordId = &orderID
	return r
}

func (r *CancelOrderRequest) ClientOrderID(clientOrderID string) *CancelOrderRequest {
	r.clOrdId = &clientOrderID
	return r
}

func (r *CancelOrderRequest) Parameters() map[string]interface{} {
	var payload = map[string]interface{}{
		"instId": r.instId,
	}

	if r.ordId != nil {
		payload["ordId"] = r.ordId
	} else if r.clOrdId != nil {
		payload["clOrdId"] = r.clOrdId
	}

	return payload
}

func (r *CancelOrderRequest) Do(ctx context.Context) ([]OrderResponse, error) {
	var payload = r.Parameters()

	if r.ordId == nil && r.clOrdId != nil {
		return nil, errors.New("either orderID or clientOrderID is required for canceling order")
	}

	req, err := r.client.newAuthenticatedRequest("POST", "/api/v5/trade/cancel-order", nil, payload)
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

type BatchCancelOrderRequest struct {
	client *RestClient

	reqs []*CancelOrderRequest
}

func (r *BatchCancelOrderRequest) Add(reqs ...*CancelOrderRequest) *BatchCancelOrderRequest {
	r.reqs = append(r.reqs, reqs...)
	return r
}

func (r *BatchCancelOrderRequest) Do(ctx context.Context) (*OrderResponse, error) {
	var parameterList []map[string]interface{}

	for _, req := range r.reqs {
		params := req.Parameters()
		parameterList = append(parameterList, params)
	}

	req, err := r.client.newAuthenticatedRequest("POST", "/api/v5/trade/cancel-batch-orders", nil, parameterList)
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

	if len(orderResponse.Data) == 0 {
		return nil, errors.New("order create error")
	}

	return &orderResponse.Data[0], nil
}

type BatchPlaceOrderRequest struct {
	client *RestClient

	reqs []*PlaceOrderRequest
}

func (r *BatchPlaceOrderRequest) Add(reqs ...*PlaceOrderRequest) *BatchPlaceOrderRequest {
	r.reqs = append(r.reqs, reqs...)
	return r
}

func (r *BatchPlaceOrderRequest) Do(ctx context.Context) ([]OrderResponse, error) {
	var parameterList []map[string]interface{}

	for _, req := range r.reqs {
		params := req.Parameters()
		parameterList = append(parameterList, params)
	}

	req, err := r.client.newAuthenticatedRequest("POST", "/api/v5/trade/batch-orders", nil, parameterList)
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

type PlaceOrderRequest struct {
	client *RestClient

	instId string

	// tdMode
	// margin mode: "cross", "isolated"
	// non-margin mode cash
	tdMode string

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string

	// "buy" or "sell"
	side SideType

	ordType OrderType

	// sz Quantity
	sz string

	// price
	px *string
}

func (r *PlaceOrderRequest) InstrumentID(instID string) *PlaceOrderRequest {
	r.instId = instID
	return r
}

func (r *PlaceOrderRequest) TradeMode(mode string) *PlaceOrderRequest {
	r.tdMode = mode
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
	r.sz = quantity
	return r
}

func (r *PlaceOrderRequest) Price(price string) *PlaceOrderRequest {
	r.px = &price
	return r
}

func (r *PlaceOrderRequest) OrderType(orderType OrderType) *PlaceOrderRequest {
	r.ordType = orderType
	return r
}

func (r *PlaceOrderRequest) Parameters() map[string]interface{} {
	payload := map[string]interface{}{}

	payload["instId"] = r.instId

	if r.tdMode == "" {
		payload["tdMode"] = "cash"
	} else {
		payload["tdMode"] = r.tdMode
	}

	if r.clientOrderID != nil {
		payload["clOrdId"] = r.clientOrderID
	}

	payload["side"] = r.side
	payload["ordType"] = r.ordType
	payload["sz"] = r.sz
	if r.px != nil {
		payload["px"] = r.px
	}

	if r.tag != nil {
		payload["tag"] = r.tag
	}

	return payload
}

func (r *PlaceOrderRequest) Do(ctx context.Context) (*OrderResponse, error) {
	payload := r.Parameters()
	req, err := r.client.newAuthenticatedRequest("POST", "/api/v5/trade/order", nil, payload)
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

	if len(orderResponse.Data) == 0 {
		return nil, errors.New("order create error")
	}

	return &orderResponse.Data[0], nil
}

type OrderDetailsRequest struct {
	client *RestClient

	instId  string
	ordId   *string
	clOrdId *string
}

func (r *OrderDetailsRequest) Parameters() map[string]interface{} {
	var payload = map[string]interface{}{
		"instId": r.instId,
	}

	if r.ordId != nil {
		payload["ordId"] = r.ordId
	} else if r.clOrdId != nil {
		payload["clOrdId"] = r.clOrdId
	}

	return payload
}

type OrderDetails struct {
	InstrumentType string           `json:"instType"`
	InstrumentID   string           `json:"instId"`
	Currency       string           `json:"ccy"`
	Tag            string           `json:"tag"`
	Price          fixedpoint.Value `json:"px"`
	Quantity       fixedpoint.Value `json:"sz"`

	// Accumulated fill quantity
	FilledQuantity fixedpoint.Value `json:"accFillSz"`

	LastFilledPrice    fixedpoint.Value `json:"fillPx"`
	LastFilledQuantity fixedpoint.Value `json:"fillSz"`

	// Average filled price. If none is filled, it will return 0.
	AveragePrice fixedpoint.Value `json:"avgPx"`

	Leverage fixedpoint.Value `json:"lever"`

	FeeCurrency string           `json:"feeCcy"`
	Fee         fixedpoint.Value `json:"fee"`

	RebateCurrency string           `json:"rebateCcy"`
	Rebate         fixedpoint.Value `json:"rebate"`

	PnL       fixedpoint.Value `json:"pnl"`
	OrderType OrderType        `json:"ordType"`
	Side      SideType         `json:"side"`

	UpdateTime   types.MillisecondTimestamp `json:"uTime"`
	CreationTime types.MillisecondTimestamp `json:"cTime"`

	State string `json:"state"`
}

func (r *OrderDetailsRequest) InstrumentID(instId string) *OrderDetailsRequest {
	r.instId = instId
	return r
}

func (r *OrderDetailsRequest) OrderID(orderID string) *OrderDetailsRequest {
	r.ordId = &orderID
	return r
}

func (r *OrderDetailsRequest) ClientOrderID(clientOrderID string) *OrderDetailsRequest {
	r.clOrdId = &clientOrderID
	return r
}

func (r *OrderDetailsRequest) Do(ctx context.Context) (*OrderDetails, error) {
	payload := r.Parameters()
	req, err := r.client.newAuthenticatedRequest("GET", "/api/v5/trade/order", nil, payload)
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

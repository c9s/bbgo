package okexapi

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

type OrderResponse struct {
	OrderID       string `json:"ordId"`
	ClientOrderID string `json:"clOrdId"`
	Tag           string `json:"tag"`
	Code          string `json:"sCode"`
	Message       string `json:"sMsg"`
}

func (c *RestClient) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{
		client: c,
	}
}

func (c *RestClient) NewBatchPlaceOrderRequest() *BatchPlaceOrderRequest {
	return &BatchPlaceOrderRequest{
		client: c,
	}
}

func (c *RestClient) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{
		client: c,
	}
}

func (c *RestClient) NewBatchCancelOrderRequest() *BatchCancelOrderRequest {
	return &BatchCancelOrderRequest{
		client: c,
	}
}

func (c *RestClient) NewGetOrderDetailsRequest() *GetOrderDetailsRequest {
	return &GetOrderDetailsRequest{
		client: c,
	}
}

func (c *RestClient) NewGetPendingOrderRequest() *GetPendingOrderRequest {
	return &GetPendingOrderRequest{
		client: c,
	}
}

func (c *RestClient) NewGetTransactionDetailsRequest() *GetTransactionDetailsRequest {
	return &GetTransactionDetailsRequest{
		client: c,
	}
}

//go:generate requestgen -type PlaceOrderRequest
type PlaceOrderRequest struct {
	client *RestClient

	instrumentID string `param:"instId"`

	// tdMode
	// margin mode: "cross", "isolated"
	// non-margin mode cash
	tradeMode string `param:"tdMode" validValues:"cross,isolated,cash"`

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string `param:"clOrdId"`

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string `param:"tag"`

	// "buy" or "sell"
	side SideType `param:"side" validValues:"buy,sell"`

	orderType OrderType `param:"ordType"`

	quantity string `param:"sz"`

	// price
	price *string `param:"px"`
}

func (r *PlaceOrderRequest) Parameters() map[string]interface{} {
	params, _ := r.GetParameters()
	return params
}

func (r *PlaceOrderRequest) Do(ctx context.Context) (*OrderResponse, error) {
	payload := r.Parameters()
	req, err := r.client.NewAuthenticatedRequest(ctx, "POST", "/api/v5/trade/order", nil, payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data []OrderResponse
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, errors.New("order create error")
	}

	return &data[0], nil
}

//go:generate requestgen -type CancelOrderRequest
type CancelOrderRequest struct {
	client *RestClient

	instrumentID  string  `param:"instId"`
	orderID       *string `param:"ordId"`
	clientOrderID *string `param:"clOrdId"`
}

func (r *CancelOrderRequest) Parameters() map[string]interface{} {
	payload, _ := r.GetParameters()
	return payload
}

func (r *CancelOrderRequest) Do(ctx context.Context) ([]OrderResponse, error) {
	payload, err := r.GetParameters()
	if err != nil {
		return nil, err
	}

	if r.clientOrderID == nil && r.orderID != nil {
		return nil, errors.New("either orderID or clientOrderID is required for canceling order")
	}

	req, err := r.client.NewAuthenticatedRequest(ctx, "POST", "/api/v5/trade/cancel-order", nil, payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data []OrderResponse
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	return data, nil
}

type BatchCancelOrderRequest struct {
	client *RestClient

	reqs []*CancelOrderRequest
}

func (r *BatchCancelOrderRequest) Add(reqs ...*CancelOrderRequest) *BatchCancelOrderRequest {
	r.reqs = append(r.reqs, reqs...)
	return r
}

func (r *BatchCancelOrderRequest) Do(ctx context.Context) ([]OrderResponse, error) {
	var parameterList []map[string]interface{}

	for _, req := range r.reqs {
		params := req.Parameters()
		parameterList = append(parameterList, params)
	}

	req, err := r.client.NewAuthenticatedRequest(ctx, "POST", "/api/v5/trade/cancel-batch-orders", nil, parameterList)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data []OrderResponse
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	return data, nil
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

	req, err := r.client.NewAuthenticatedRequest(ctx, "POST", "/api/v5/trade/batch-orders", nil, parameterList)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data []OrderResponse
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	return data, nil
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
	req, err := r.client.NewAuthenticatedRequest(ctx, "GET", "/api/v5/trade/order", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data []OrderDetails
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, errors.New("get order details error")
	}

	return &data[0], nil
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
	req, err := r.client.NewAuthenticatedRequest(ctx, "GET", "/api/v5/trade/orders-pending", nil, payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data []OrderDetails
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	return data, nil
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
	req, err := r.client.NewAuthenticatedRequest(ctx, "GET", "/api/v5/trade/fills", nil, payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data []OrderDetails
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	return data, nil
}

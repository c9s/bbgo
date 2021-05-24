package okexapi

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
	OrderID       string `json:"ordId"`
	ClientOrderID string `json:"clOrdId"`
	Tag           string `json:"tag"`
	Code          string `json:"sCode"`
	Message       string `json:"sMsg"`
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

func (c *TradeService) NewBatchCancelOrderRequest() *BatchCancelOrderRequest {
	return &BatchCancelOrderRequest{
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

	instType *string

	orderTypes []string

	state *OrderState
}

func (r *GetPendingOrderRequest) InstrumentType(instType string) *GetPendingOrderRequest {
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

	instType *string

	instId *string

	ordId *string
}

func (r *GetTransactionDetailsRequest) InstrumentType(instType string) *GetTransactionDetailsRequest {
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

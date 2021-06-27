package max

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

var relUrlV2Order *url.URL
var relUrlV2Orders *url.URL
var relUrlV2OrdersClear *url.URL
var relUrlV2OrdersDelete *url.URL
var relUrlV2OrdersMultiOneByOne, relUrlV2OrderDelete *url.URL

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func init() {
	relUrlV2Order = mustParseURL("v2/order")
	relUrlV2OrderDelete = mustParseURL("v2/order/delete")
	relUrlV2Orders = mustParseURL("v2/orders")
	relUrlV2OrdersClear = mustParseURL("v2/orders/clear")
	relUrlV2OrdersDelete = mustParseURL("v2/orders/delete")
	relUrlV2OrdersMultiOneByOne = mustParseURL("v2/orders/multi/onebyone")
}

type OrderStateToQuery int

const (
	All = iota
	Active
	Closed
)

type OrderState string

const (
	OrderStateDone       = OrderState("done")
	OrderStateCancel     = OrderState("cancel")
	OrderStateWait       = OrderState("wait")
	OrderStateConvert    = OrderState("convert")
	OrderStateFinalizing = OrderState("finalizing")
	OrderStateFailed     = OrderState("failed")
)

type OrderType string

// Order types that the API can return.
const (
	OrderTypeMarket     = OrderType("market")
	OrderTypeLimit      = OrderType("limit")
	OrderTypePostOnly   = OrderType("post_only")
	OrderTypeStopLimit  = OrderType("stop_limit")
	OrderTypeStopMarket = OrderType("stop_market")
	OrderTypeIOCLimit   = OrderType("ioc_limit")
)

type QueryOrderOptions struct {
	GroupID int
	Offset  int
	Limit   int
}

// OrderService manages the Order endpoint.
type OrderService struct {
	client *RestClient
}

// Order represents one returned order (POST order/GET order/GET orders) on the max platform.
type Order struct {
	ID              uint64     `json:"id,omitempty"`
	Side            string     `json:"side"`
	OrderType       OrderType  `json:"ord_type"`
	Price           string     `json:"price,omitempty"`
	StopPrice       string     `json:"stop_price,omitempty"`
	AveragePrice    string     `json:"avg_price,omitempty"`
	State           OrderState `json:"state,omitempty"`
	Market          string     `json:"market,omitempty"`
	Volume          string     `json:"volume"`
	RemainingVolume string     `json:"remaining_volume,omitempty"`
	ExecutedVolume  string     `json:"executed_volume,omitempty"`
	TradesCount     int64      `json:"trades_count,omitempty"`
	GroupID         uint32     `json:"group_id,omitempty"`
	ClientOID       string     `json:"client_oid,omitempty"`
	CreatedAt       time.Time  `json:"-" db:"created_at"`
	CreatedAtMs     int64      `json:"created_at_in_ms,omitempty"`
	InsertedAt      time.Time  `json:"-" db:"inserted_at"`
}

// Open returns open orders
func (s *OrderService) Closed(market string, options QueryOrderOptions) ([]Order, error) {
	payload := map[string]interface{}{
		"market":     market,
		"state":      []OrderState{OrderStateDone, OrderStateCancel, OrderStateFailed},
		"order_by":   "desc",
		"pagination": false,
	}

	if options.GroupID > 0 {
		payload["group_id"] = options.GroupID
	}
	if options.Offset > 0 {
		payload["offset"] = options.Offset
	}
	if options.Limit > 0 {
		payload["limit"] = options.Limit
	}

	req, err := s.client.newAuthenticatedRequest("GET", "v2/orders", payload, relUrlV2Orders)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var orders []Order
	if err := response.DecodeJSON(&orders); err != nil {
		return nil, err
	}

	return orders, nil
}

// Open returns open orders
func (s *OrderService) Open(market string, options QueryOrderOptions) ([]Order, error) {
	payload := map[string]interface{}{
		"market": market,
		// "state":    []OrderState{OrderStateWait, OrderStateConvert},
		"order_by":   "desc",
		"pagination": false,
	}

	if options.GroupID > 0 {
		payload["group_id"] = options.GroupID
	}

	req, err := s.client.newAuthenticatedRequest("GET", "v2/orders", payload, relUrlV2Orders)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var orders []Order
	if err := response.DecodeJSON(&orders); err != nil {
		return nil, err
	}

	return orders, nil
}

// All returns all orders for the authenticated account.
func (s *OrderService) All(market string, limit, page int, states ...OrderState) ([]Order, error) {
	payload := map[string]interface{}{
		"market":   market,
		"limit":    limit,
		"page":     page,
		"state":    states,
		"order_by": "desc",
	}

	req, err := s.client.newAuthenticatedRequest("GET", "v2/orders", payload, relUrlV2Orders)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var orders []Order
	if err := response.DecodeJSON(&orders); err != nil {
		return nil, err
	}

	return orders, nil
}

// CancelAll active orders for the authenticated account.
func (s *OrderService) CancelAll(side string, market string) error {
	payload := map[string]interface{}{}
	if side == "buy" || side == "sell" {
		payload["side"] = side
	}
	if market != "all" {
		payload["market"] = market
	}

	req, err := s.client.newAuthenticatedRequest("POST", "v2/orders/clear", payload, relUrlV2OrdersClear)
	if err != nil {
		return err
	}

	_, err = s.client.sendRequest(req)
	if err != nil {
		return err
	}

	return nil
}

// Options carry the option fields for REST API
type Options map[string]interface{}

// Create a new order.
func (s *OrderService) Create(market string, side string, volume float64, price float64, orderType string, options Options) (*Order, error) {
	options["market"] = market
	options["volume"] = strconv.FormatFloat(volume, 'f', -1, 64)
	options["price"] = strconv.FormatFloat(price, 'f', -1, 64)
	options["side"] = side
	options["ord_type"] = orderType
	response, err := s.client.sendAuthenticatedRequest("POST", "v2/orders", options)
	if err != nil {
		return nil, err
	}

	var order = Order{}
	if err := response.DecodeJSON(&order); err != nil {
		return nil, err
	}

	return &order, nil
}

// Create multiple order in a single request
func (s *OrderService) CreateMulti(market string, orders []Order) (*MultiOrderResponse, error) {
	req := s.NewCreateMultiOrderRequest()
	req.Market(market)
	req.AddOrders(orders...)
	return req.Do(context.Background())
}

// Cancel the order with id `orderID`.
func (s *OrderService) Cancel(orderID uint64, clientOrderID string) error {
	req := s.NewOrderCancelRequest()

	if orderID > 0 {
		req.ID(orderID)
	} else if len(clientOrderID) > 0 {
		req.ClientOrderID(clientOrderID)
	}

	return req.Do(context.Background())
}

type OrderCancelAllRequestParams struct {
	*PrivateRequestParams

	Side    string `json:"side,omitempty"`
	Market  string `json:"market,omitempty"`
	GroupID int64  `json:"groupID,omitempty"`
}

type OrderCancelAllRequest struct {
	client *RestClient

	params OrderCancelAllRequestParams

	side    *string
	market  *string
	groupID *uint32
}

func (r *OrderCancelAllRequest) Side(side string) *OrderCancelAllRequest {
	r.side = &side
	return r
}

func (r *OrderCancelAllRequest) Market(market string) *OrderCancelAllRequest {
	r.market = &market
	return r
}

func (r *OrderCancelAllRequest) GroupID(groupID uint32) *OrderCancelAllRequest {
	r.groupID = &groupID
	return r
}

func (r *OrderCancelAllRequest) Do(ctx context.Context) (orders []Order, err error) {
	var payload = map[string]interface{}{}
	if r.side != nil {
		payload["side"] = *r.side
	}
	if r.market != nil {
		payload["market"] = *r.market
	}
	if r.groupID != nil {
		payload["groupID"] = *r.groupID
	}

	req, err := r.client.newAuthenticatedRequest("POST", "v2/orders/clear", payload, nil)
	if err != nil {
		return
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return
	}

	err = response.DecodeJSON(&orders)
	return
}

func (s *OrderService) NewOrderCancelAllRequest() *OrderCancelAllRequest {
	return &OrderCancelAllRequest{client: s.client}
}

type OrderCancelRequestParams struct {
	*PrivateRequestParams

	ID            uint64 `json:"id,omitempty"`
	ClientOrderID string `json:"client_oid,omitempty"`
}

type OrderCancelRequest struct {
	client *RestClient

	params OrderCancelRequestParams
}

func (r *OrderCancelRequest) ID(id uint64) *OrderCancelRequest {
	r.params.ID = id
	return r
}

func (r *OrderCancelRequest) ClientOrderID(id string) *OrderCancelRequest {
	r.params.ClientOrderID = id
	return r
}

func (r *OrderCancelRequest) Do(ctx context.Context) error {
	req, err := r.client.newAuthenticatedRequest("POST", "v2/order/delete", &r.params, relUrlV2OrderDelete)
	if err != nil {
		return err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return err
	}

	var order = Order{}
	if err := response.DecodeJSON(&order); err != nil {
		return err
	}

	return err
}

func (s *OrderService) NewOrderCancelRequest() *OrderCancelRequest {
	return &OrderCancelRequest{client: s.client}
}

// Status retrieves the given order from the API.
func (s *OrderService) Get(orderID uint64) (*Order, error) {
	payload := map[string]interface{}{
		"id": orderID,
	}

	req, err := s.client.newAuthenticatedRequest("GET", "v2/order", payload, relUrlV2Order)
	if err != nil {
		return &Order{}, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var order = Order{}

	if err := response.DecodeJSON(&order); err != nil {
		return nil, err
	}

	return &order, nil
}

type MultiOrderRequestParams struct {
	*PrivateRequestParams

	Market string  `json:"market"`
	Orders []Order `json:"orders"`
}

type MultiOrderResponse []struct {
	Error string `json:"error,omitempty"`
	Order Order  `json:"order,omitempty"`
}

type CreateMultiOrderRequest struct {
	client *RestClient

	market  *string
	groupID *uint32
	orders  []Order
}

func (r *CreateMultiOrderRequest) GroupID(groupID uint32) *CreateMultiOrderRequest {
	r.groupID = &groupID
	return r
}

func (r *CreateMultiOrderRequest) Market(market string) *CreateMultiOrderRequest {
	r.market = &market
	return r
}

func (r *CreateMultiOrderRequest) AddOrders(orders ...Order) *CreateMultiOrderRequest {
	r.orders = append(r.orders, orders...)
	return r
}

func (r *CreateMultiOrderRequest) Do(ctx context.Context) (multiOrderResponse *MultiOrderResponse, err error) {
	var payload = map[string]interface{}{}

	if r.market != nil {
		payload["market"] = r.market
	} else {
		return nil, errors.New("parameter market is required")
	}

	if r.groupID != nil {
		payload["group_id"] = r.groupID
	}

	if len(r.orders) == 0 {
		return nil, errors.New("parameter orders can not be empty")
	}

	// clear group id
	for i := range r.orders {
		r.orders[i].GroupID = 0
	}

	payload["orders"] = r.orders

	req, err := r.client.newAuthenticatedRequest("POST", "v2/orders/multi/onebyone", payload, relUrlV2OrdersMultiOneByOne)
	if err != nil {
		return multiOrderResponse, errors.Wrapf(err, "order create error")
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return multiOrderResponse, err
	}

	multiOrderResponse = &MultiOrderResponse{}
	if errJson := response.DecodeJSON(multiOrderResponse); errJson != nil {
		return multiOrderResponse, errJson
	}

	return multiOrderResponse, err
}

func (s *OrderService) NewCreateMultiOrderRequest() *CreateMultiOrderRequest {
	return &CreateMultiOrderRequest{client: s.client}
}

type CreateOrderRequest struct {
	client *RestClient

	market        *string
	volume        *string
	price         *string
	stopPrice     *string
	side          *string
	orderType     *string
	clientOrderID *string
	groupID       *string
}

func (r *CreateOrderRequest) Market(market string) *CreateOrderRequest {
	r.market = &market
	return r
}

func (r *CreateOrderRequest) Volume(volume string) *CreateOrderRequest {
	r.volume = &volume
	return r
}

func (r *CreateOrderRequest) Price(price string) *CreateOrderRequest {
	r.price = &price
	return r
}

func (r *CreateOrderRequest) StopPrice(price string) *CreateOrderRequest {
	r.stopPrice = &price
	return r
}

func (r *CreateOrderRequest) Side(side string) *CreateOrderRequest {
	r.side = &side
	return r
}

func (r *CreateOrderRequest) OrderType(orderType string) *CreateOrderRequest {
	r.orderType = &orderType
	return r
}

func (r *CreateOrderRequest) ClientOrderID(clientOrderID string) *CreateOrderRequest {
	r.clientOrderID = &clientOrderID
	return r
}

func (r *CreateOrderRequest) Do(ctx context.Context) (order *Order, err error) {
	var payload = map[string]interface{}{
		"market": r.market,
		"volume": r.volume,
		"side":   r.side,
	}

	if r.price != nil {
		payload["price"] = r.price
	}

	if r.stopPrice != nil {
		payload["stop_price"] = r.stopPrice
	}

	if r.orderType != nil {
		payload["ord_type"] = r.orderType
	}

	if r.clientOrderID != nil {
		payload["client_oid"] = r.clientOrderID
	}

	if r.groupID != nil {
		payload["group_id"] = r.groupID
	}

	req, err := r.client.newAuthenticatedRequest("POST", "v2/orders", payload, relUrlV2Orders)
	if err != nil {
		return order, errors.Wrapf(err, "order create error")
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return order, err
	}

	order = &Order{}
	if err := response.DecodeJSON(order); err != nil {
		return nil, err
	}

	return order, err
}

func (s *OrderService) NewCreateOrderRequest() *CreateOrderRequest {
	return &CreateOrderRequest{client: s.client}
}

package max

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST

import (
	"context"
	"net/url"
	"time"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/types"
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
	OrderStateDone = OrderState("done")

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
	Page    int
	OrderBy string
}

// OrderService manages the Order endpoint.
type OrderService struct {
	client *RestClient
}

// Order represents one returned order (POST order/GET order/GET orders) on the max platform.
type Order struct {
	ID              uint64                     `json:"id,omitempty"`
	Side            string                     `json:"side"`
	OrderType       OrderType                  `json:"ord_type"`
	Price           string                     `json:"price,omitempty"`
	StopPrice       string                     `json:"stop_price,omitempty"`
	AveragePrice    string                     `json:"avg_price,omitempty"`
	State           OrderState                 `json:"state,omitempty"`
	Market          string                     `json:"market,omitempty"`
	Volume          string                     `json:"volume"`
	RemainingVolume string                     `json:"remaining_volume,omitempty"`
	ExecutedVolume  string                     `json:"executed_volume,omitempty"`
	TradesCount     int64                      `json:"trades_count,omitempty"`
	GroupID         uint32                     `json:"group_id,omitempty"`
	ClientOID       string                     `json:"client_oid,omitempty"`
	CreatedAt       time.Time                  `json:"-" db:"created_at"`
	CreatedAtMs     types.MillisecondTimestamp `json:"created_at_in_ms,omitempty"`
	InsertedAt      time.Time                  `json:"-" db:"inserted_at"`
}

// Open returns open orders
func (s *OrderService) Closed(market string, options QueryOrderOptions) ([]Order, error) {
	req := s.NewGetOrdersRequest()
	req.Market(market)
	req.State([]OrderState{OrderStateDone, OrderStateCancel})

	if options.GroupID > 0 {
		req.GroupID(uint32(options.GroupID))
	}
	if options.Offset > 0 {
		req.Offset(options.Offset)
	}
	if options.Limit > 0 {
		req.Limit(options.Limit)
	}

	if options.Page > 0 {
		req.Page(options.Page)
	}

	if len(options.OrderBy) > 0 {
		req.OrderBy(options.OrderBy)
	}

	return req.Do(context.Background())
}

// Open returns open orders
func (s *OrderService) Open(market string, options QueryOrderOptions) ([]Order, error) {
	req := s.NewGetOrdersRequest()
	req.Market(market)
	// state default ot wait and convert

	if options.GroupID > 0 {
		req.GroupID(uint32(options.GroupID))
	}

	return req.Do(context.Background())
}

//go:generate GetRequest -url "v2/orders/history" -type GetOrderHistoryRequest -responseType []Order
type GetOrderHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	market string `param:"market"`
	fromID *uint64 `param:"from_id"`
	limit  *uint   `param:"limit"`
}

func (s *OrderService) NewGetOrderHistoryRequest() *GetOrderHistoryRequest {
	return &GetOrderHistoryRequest{
		client: s.client,
	}
}

//go:generate GetRequest -url "v2/orders" -type GetOrdersRequest -responseType []Order
type GetOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	market  string       `param:"market"`
	side    *string      `param:"side"`
	groupID *uint32      `param:"groupID"`
	offset  *int         `param:"offset"`
	limit   *int         `param:"limit"`
	page    *int         `param:"page"`
	orderBy *string      `param:"order_by" default:"desc"`
	state   []OrderState `param:"state"`
}

func (s *OrderService) NewGetOrdersRequest() *GetOrdersRequest {
	return &GetOrdersRequest{
		client: s.client,
	}
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

	req, err := s.client.newAuthenticatedRequest(context.Background(), "GET", "v2/orders", nil, payload, relUrlV2Orders)
	if err != nil {
		return nil, err
	}

	response, err := s.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var orders []Order
	if err := response.DecodeJSON(&orders); err != nil {
		return nil, err
	}

	return orders, nil
}

// Options carry the option fields for REST API
type Options map[string]interface{}

// Create multiple order in a single request
func (s *OrderService) CreateMulti(market string, orders []Order) (*MultiOrderResponse, error) {
	req := s.NewCreateMultiOrderRequest()
	req.Market(market)
	req.AddOrders(orders...)
	return req.Do(context.Background())
}

//go:generate PostRequest -url "v2/orders/clear" -type OrderCancelAllRequest -responseType []Order
type OrderCancelAllRequest struct {
	client requestgen.AuthenticatedAPIClient

	side    *string `param:"side"`
	market  *string `param:"market"`
	groupID *uint32 `param:"groupID"`
}

func (s *OrderService) NewOrderCancelAllRequest() *OrderCancelAllRequest {
	return &OrderCancelAllRequest{client: s.client}
}

//go:generate PostRequest -url "v2/order/delete" -type OrderCancelRequest -responseType .Order
type OrderCancelRequest struct {
	client requestgen.AuthenticatedAPIClient

	id            *uint64 `param:"id,omitempty"`
	clientOrderID *string `param:"client_oid,omitempty"`
}

func (s *OrderService) NewOrderCancelRequest() *OrderCancelRequest {
	return &OrderCancelRequest{client: s.client}
}

//go:generate GetRequest -url "v2/order" -type GetOrderRequest -responseType .Order
type GetOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	id            *uint64 `param:"id,omitempty"`
	clientOrderID *string `param:"client_oid,omitempty"`
}

func (s *OrderService) NewGetOrderRequest() *GetOrderRequest {
	return &GetOrderRequest{client: s.client}
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

	req, err := r.client.newAuthenticatedRequest(context.Background(), "POST", "v2/orders/multi/onebyone", nil, payload, relUrlV2OrdersMultiOneByOne)
	if err != nil {
		return multiOrderResponse, errors.Wrapf(err, "order create error")
	}

	response, err := r.client.SendRequest(req)
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

//go:generate PostRequest -url "v2/orders" -type CreateOrderRequest -responseType .Order
type CreateOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	market    string `param:"market,required"`
	side      string `param:"side,required"`
	volume    string `param:"volume,required"`
	orderType string `param:"ord_type"`

	price         *string `param:"price"`
	stopPrice     *string `param:"stop_price"`
	clientOrderID *string `param:"client_oid"`
	groupID       *string `param:"group_id"`
}

func (s *OrderService) NewCreateOrderRequest() *CreateOrderRequest {
	return &CreateOrderRequest{client: s.client}
}

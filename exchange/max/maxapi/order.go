package max

import (
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

type OrderStateToQuery int

const (
	All = iota
	Active
	Closed
)

type OrderType string

// Order types that the API can return.
const (
	OrderTypeMarket = OrderType("market")
	OrderTypeLimit  = OrderType("limit")
)

// OrderService manages the Order endpoint.
type OrderService struct {
	client *RestClient
}

// Order represents one returned order (POST order/GET order/GET orders) on the max platform.
type Order struct {
	ID              uint64    `json:"id,omitempty" db:"exchange_id"`
	Side            string    `json:"side" db:"side"`
	OrderType       string    `json:"ord_type,omitempty" db:"order_type"`
	Price           string    `json:"price" db:"price"`
	AveragePrice    string    `json:"avg_price,omitempty" db:"average_price"`
	State           string    `json:"state,omitempty" db:"state"`
	Market          string    `json:"market,omitempty" db:"market"`
	Volume          string    `json:"volume" db:"volume"`
	RemainingVolume string    `json:"remaining_volume,omitempty" db:"remaining_volume"`
	ExecutedVolume  string    `json:"executed_volume,omitempty" db:"executed_volume"`
	TradesCount     int64     `json:"trades_count,omitempty" db:"trades_count"`
	GroupID         int64     `json:"group_id,omitempty" db:"group_id"`
	ClientOID       string    `json:"client_oid,omitempty" db:"client_oid"`
	CreatedAt       time.Time `json:"-" db:"created_at"`
	CreatedAtMs     int64     `json:"created_at_in_ms,omitempty"`
	InsertedAt      time.Time `json:"-" db:"inserted_at"`
}

// All returns all orders for the authenticated account.
func (s *OrderService) All(market string, limit, page int, state OrderStateToQuery) ([]Order, error) {
	var states []string
	switch state {
	case All:
		states = []string{"done", "cancel", "wait", "convert"}
	case Active:
		states = []string{"wait", "convert"}
	case Closed:
		states = []string{"done", "cancel"}
	default:
		states = []string{"wait", "convert"}
	}

	payload := map[string]interface{}{
		"market":   market,
		"limit":    limit,
		"page":     page,
		"state":    states,
		"order_by": "desc",
	}
	req, err := s.client.newAuthenticatedRequest("GET", "v2/orders", payload)
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

	req, err := s.client.newAuthenticatedRequest("POST", "v2/orders/clear", payload)
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

type OrderCancelRequestParams struct {
	ID            int    `json:"id"`
	ClientOrderID string `json:"client_oid"`
}

func (p OrderCancelRequestParams) Map() map[string]interface{} {
	payload := make(map[string]interface{})

	if p.ID > 0 {
		payload["id"] = p.ID
	}

	if len(p.ClientOrderID) > 0 {
		payload["client_oid"] = p.ClientOrderID
	}

	return payload
}

type OrderCancelRequest struct {
	client *RestClient

	params OrderCancelRequestParams
}

func (r *OrderCancelRequest) ID(id int) *OrderCancelRequest {
	r.params.ID = id
	return r
}

func (r *OrderCancelRequest) ClientOrderID(id string) *OrderCancelRequest {
	r.params.ClientOrderID = id
	return r
}

func (r *OrderCancelRequest) Do(ctx context.Context) error {
	payload := r.params.Map()
	req, err := r.client.newAuthenticatedRequest("POST", "v2/order/delete", payload)
	if err != nil {
		return err
	}

	_, err = r.client.sendRequest(req)
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

	req, err := s.client.newAuthenticatedRequest("GET", "v2/order", payload)

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
	Market string  `json:"market"`
	Orders []Order `json:"orders"`
}

func (p *MultiOrderRequestParams) Map() map[string]interface{} {
	return map[string]interface{}{
		"market": p.Market,
		"orders": p.Orders,
	}
}

type MultiOrderResponse []struct {
	Error string `json:"error,omitempty"`
	Order Order  `json:"order,omitempty"`
}

type CreateMultiOrderRequest struct {
	client *RestClient

	params MultiOrderRequestParams
}

func (r *CreateMultiOrderRequest) Market(market string) *CreateMultiOrderRequest {
	r.params.Market = market
	return r
}

func (r *CreateMultiOrderRequest) AddOrders(orders ...Order) *CreateMultiOrderRequest {
	r.params.Orders = append(r.params.Orders, orders...)
	return r
}

func (r *CreateMultiOrderRequest) Do(ctx context.Context) (multiOrderResponse *MultiOrderResponse, err error) {
	req, err := r.client.newAuthenticatedRequest("POST", "v2/orders/multi/onebyone", r.params.Map())
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

// Create multiple order in a single request
func (s *OrderService) CreateMulti(market string, orders []Order) (*MultiOrderResponse, error) {
	req := s.NewCreateMultiOrderRequest()
	req.Market(market)
	req.AddOrders(orders...)
	return req.Do(context.Background())
}

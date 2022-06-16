package max

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST

import (
	"context"
	"net/url"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var (
	relUrlV2Orders              *url.URL
	relUrlV2OrdersMultiOneByOne *url.URL
)

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func init() {
	relUrlV2Orders = mustParseURL("v2/orders")
	relUrlV2OrdersMultiOneByOne = mustParseURL("v2/orders/multi/onebyone")
}

type WalletType string

const (
	WalletTypeSpot   WalletType = "spot"
	WalletTypeMargin WalletType = "m"
)

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

type SubmitOrder struct {
	Side      string    `json:"side"`
	Market    string    `json:"market"`
	Price     string    `json:"price"`
	StopPrice string    `json:"stop_price,omitempty"`
	OrderType OrderType `json:"ord_type"`
	Volume    string    `json:"volume"`
	GroupID   uint32    `json:"group_id,omitempty"`
	ClientOID string    `json:"client_oid,omitempty"`
}

// Order represents one returned order (POST order/GET order/GET orders) on the max platform.
type Order struct {
	ID              uint64                     `json:"id,omitempty"`
	WalletType      WalletType                 `json:"wallet_type,omitempty"`
	Side            string                     `json:"side"`
	OrderType       OrderType                  `json:"ord_type"`
	Price           fixedpoint.Value           `json:"price,omitempty"`
	StopPrice       fixedpoint.Value           `json:"stop_price,omitempty"`
	AveragePrice    fixedpoint.Value           `json:"avg_price,omitempty"`
	State           OrderState                 `json:"state,omitempty"`
	Market          string                     `json:"market,omitempty"`
	Volume          fixedpoint.Value           `json:"volume"`
	RemainingVolume fixedpoint.Value           `json:"remaining_volume,omitempty"`
	ExecutedVolume  fixedpoint.Value           `json:"executed_volume,omitempty"`
	TradesCount     int64                      `json:"trades_count,omitempty"`
	GroupID         uint32                     `json:"group_id,omitempty"`
	ClientOID       string                     `json:"client_oid,omitempty"`
	CreatedAt       types.MillisecondTimestamp `json:"created_at"`
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
func (s *OrderService) CreateMulti(market string, orders []SubmitOrder) (*MultiOrderResponse, error) {
	req := s.NewCreateMultiOrderRequest()
	req.Market(market)
	req.AddOrders(orders...)
	return req.Do(context.Background())
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
	orders  []SubmitOrder
}

func (r *CreateMultiOrderRequest) GroupID(groupID uint32) *CreateMultiOrderRequest {
	r.groupID = &groupID
	return r
}

func (r *CreateMultiOrderRequest) Market(market string) *CreateMultiOrderRequest {
	r.market = &market
	return r
}

func (r *CreateMultiOrderRequest) AddOrders(orders ...SubmitOrder) *CreateMultiOrderRequest {
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


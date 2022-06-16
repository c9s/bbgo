package max

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST

import (
	"context"
	"net/url"

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

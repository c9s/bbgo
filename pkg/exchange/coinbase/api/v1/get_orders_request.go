package coinbase

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

func (s *OrderStatus) GlobalOrderStatus() types.OrderStatus {
	switch *s {
	case OrderStatusRejected:
		return types.OrderStatusRejected
	case OrderStatusOpen, OrderStatusPending, OrderStatusDone, OrderStatusActive, OrderStatusReceived, OrderStatusAll:
		return types.OrderStatus(strings.ToUpper(string(*s)))
	}
	return types.OrderStatus(strings.ToUpper(string(*s)))
}

type Order struct {
	Type      string      `json:"type"`
	Size      json.Number `json:"size"`
	Side      string      `json:"side"`
	ProductID string      `json:"product_id"`
	// ClientOID must be uuid
	ClientOID string      `json:"client_oid"`
	Stop      string      `json:"stop"`
	StopPrice json.Number `json:"stop_price"`

	// Limit Order
	Price       json.Number `json:"price"`
	TimeInForce string      `json:"time_in_force"`
	PostOnly    bool        `json:"post_only"`
	CancelAfter string      `json:"cancel_after"`

	// Market Order
	Funds          json.Number `json:"funds"`
	SpecifiedFunds json.Number `json:"specified_funds"`

	// Response Fields
	ID            string      `json:"id"`
	Status        OrderStatus `json:"status"`
	Settled       bool        `json:"settled"`
	DoneReason    string      `json:"done_reason"`
	DoneAt        time.Time   `json:"done_at"`
	CreatedAt     time.Time   `json:"created_at"`
	FillFees      json.Number `json:"fill_fees"`
	FilledSize    json.Number `json:"filled_size"`
	ExecutedValue json.Number `json:"executed_value"`
}

type OrderSnapshot []Order

//go:generate requestgen -method GET -url "/orders" -type GetOrdersRequest -responseType .OrderSnapshot
type GetOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	profileID *string    `param:"profile_id"`
	productID *string    `param:"product_id"`
	sortedBy  *string    `param:"sortedBy" validValues:"created_at,price,size,order_id,side,type"`
	sorting   *string    `param:"sorting" validValues:"asc,desc"`
	startDate *time.Time `param:"start_date" timeFormat:"RFC3339"`
	endDate   *time.Time `param:"end_date" timeFormat:"RFC3339"`
	before    *time.Time `param:"before" timeFormat:"RFC3339"`
	after     *time.Time `param:"after" timeFormat:"RFC3339"`
	limit     int        `param:"limit,required"`
	status    []string   `param:"status,required"`
}

//go:generate requestgen -method GET -url "/orders/:order_id" -type GetSingleOrderRequest -responseType .Order
type GetSingleOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	orderID string `param:"order_id,required"`
}

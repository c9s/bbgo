package coinbase

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type Order struct {
	Type      string           `json:"type"`
	Size      fixedpoint.Value `json:"size"`
	Side      SideType         `json:"side"`
	ProductID string           `json:"product_id"`
	// ClientOID must be uuid
	ClientOID string           `json:"client_oid"`
	Stop      string           `json:"stop"`
	StopPrice fixedpoint.Value `json:"stop_price"`

	// Limit Order
	Price       fixedpoint.Value `json:"price"`
	TimeInForce TimeInForceType  `json:"time_in_force"`
	PostOnly    bool             `json:"post_only"`
	CancelAfter string           `json:"cancel_after"`

	// Market Order
	Funds          fixedpoint.Value `json:"funds"`
	SpecifiedFunds fixedpoint.Value `json:"specified_funds"`

	// Response Fields
	ID            string           `json:"id"`
	Status        OrderStatus      `json:"status"`
	Settled       bool             `json:"settled"`
	DoneReason    string           `json:"done_reason"`
	DoneAt        types.Time       `json:"done_at"`
	CreatedAt     types.Time       `json:"created_at"`
	FillFees      fixedpoint.Value `json:"fill_fees"`
	FilledSize    fixedpoint.Value `json:"filled_size"`
	ExecutedValue fixedpoint.Value `json:"executed_value"`
}

type OrderSnapshot []Order

//go:generate requestgen -method GET -url /orders -rateLimiter 1+20/2s -type GetOrdersRequest -responseType .OrderSnapshot
type GetOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	profileID *string    `param:"profile_id"`
	productID *string    `param:"product_id"`
	sortedBy  *string    `param:"sortedBy" validValues:"created_at,price,size,order_id,side,type"`
	sorting   *string    `param:"sorting" validValues:"asc,desc"`
	startDate *time.Time `param:"start_date" timeFormat:"RFC3339"`
	endDate   *time.Time `param:"end_date" timeFormat:"RFC3339"`
	before    *time.Time `param:"before" timeFormat:"RFC3339"` // pagination id, which is the date of the order (exclusive)
	after     *time.Time `param:"after" timeFormat:"RFC3339"`  // pagination id, which is the date of the order (exclusive)
	limit     int        `param:"limit,required"`
	status    []string   `param:"status,required"`
}

//go:generate requestgen -method GET -url /orders/:order_id -rateLimiter 1+20/2s  -type GetSingleOrderRequest -responseType .Order
type GetSingleOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	orderID string `param:"order_id,slug,required"`
}

func (client *RestAPIClient) NewGetOrdersRequest() *GetOrdersRequest {
	req := GetOrdersRequest{
		client: client,
	}
	return &req
}

func (client *RestAPIClient) NewSingleOrderRequst() *GetSingleOrderRequest {
	req := GetSingleOrderRequest{
		client: client,
	}
	return &req
}

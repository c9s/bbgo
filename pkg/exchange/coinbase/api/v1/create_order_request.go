package coinbase

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type CreateOrderResponse struct {
	ID             string    `json:"id"`
	Price          string    `json:"price"`
	Size           string    `json:"size"`
	ProductID      string    `json:"product_id"`
	ProfileID      string    `json:"profile_id"`
	Side           string    `json:"side"`
	Funds          string    `json:"funds"`
	SpecifiedFunds string    `json:"specified_funds"`
	Type           string    `json:"type"`
	TimeInForce    string    `json:"time_in_force"`
	ExpireTime     time.Time `json:"expire_time"`
	PostOnly       bool      `json:"post_only"`
	CreatedAt      time.Time `json:"created_at"`
	DoneAt         time.Time `json:"done_at"`
	DoneReason     string    `json:"done_reason"`
	RejectReason   string    `json:"reject_reason"`
	FillFees       string    `json:"fill_fees"`
	FilledSize     string    `json:"filled_size"`
	ExecutedValue  string    `json:"executed_value"`
	Status         string    `json:"status"`
	Settled        bool      `json:"settled"`
	Stop           string    `json:"stop"`
	StopPrice      string    `json:"stop_price"`
	FundingAmount  string    `json:"funding_amount"`
	ClientOrderID  string    `json:"client_oid"`
}

//go:generate requestgen -method POST -url "/orders" -type CreateOrderRequest -responseType .CreateOrderResponse
type CreateOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	profileID     *string `param:"profile_id"`
	orderType     string  `param:"type,required" validValues:"limit,market,stop"`
	side          string  `param:"side,required" validValues:"buy,sell"`
	productID     string  `param:"product_id,required"`
	stp           *string `param:"stp" validValues:"dc,co,cn,cb"`
	stopPrice     *string `param:"stop_price" validValues:"loss,entry"`
	price         *string `param:"price"`
	size          string  `param:"size,required"`
	funds         *string `param:"funds"`
	timeInForce   *string `param:"time_in_force" validValues:"GTC,GCC,IOC,FOK"`
	cancelAfter   *string `param:"cancel_after" validValues:"min,hour,day"`
	postOnly      *bool   `param:"post_only"`
	clientOrderID *string `param:"client_oid"`
}

func (client *RestAPIClient) NewCreateOrderRequest(order types.SubmitOrder) *CreateOrderRequest {
	return &CreateOrderRequest{
		client:    client,
		orderType: string(order.Type),
		side:      string(order.Side),
		productID: order.Symbol,
		size:      order.Quantity.String(),
	}
}

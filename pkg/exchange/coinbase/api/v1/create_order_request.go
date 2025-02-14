package coinbase

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type CreateOrderResponse struct {
	ID               string           `json:"id"`
	Price            fixedpoint.Value `json:"price,omitempty"`
	Size             fixedpoint.Value `json:"size,omitempty"`
	ProductID        string           `json:"product_id"`
	ProfileID        string           `json:"profile_id,omitempty"`
	Side             SideType         `json:"side"`
	Funds            fixedpoint.Value `json:"funds,omitempty"`
	SpecifiedFunds   fixedpoint.Value `json:"specified_funds,omitempty"`
	Type             OrderType        `json:"type"`
	TimeInForce      TimeInForceType  `json:"time_in_force,omitempty"`
	ExpireTime       string           `json:"expire_time,omitempty"`
	PostOnly         bool             `json:"post_only"`
	CreatedAt        string           `json:"created_at"`
	DoneAt           string           `json:"done_at,omitempty"`
	DoneReason       string           `json:"done_reason,omitempty"`
	RejectReason     string           `json:"reject_reason,omitempty"`
	FillFees         fixedpoint.Value `json:"fill_fees"`
	FilledSize       fixedpoint.Value `json:"filled_size"`
	ExecutedValue    fixedpoint.Value `json:"executed_value,omitempty"`
	Status           OrderStatus      `json:"status"`
	Settled          bool             `json:"settled"`
	Stop             OrderStopType    `json:"stop,omitempty"`
	StopPrice        fixedpoint.Value `json:"stop_price,omitempty"`
	FundingAmount    string           `json:"funding_amount,omitempty"`
	ClientOrderID    string           `json:"client_oid,omitempty"`
	MarketType       string           `json:"market_type,omitempty"`
	MaxFloor         string           `json:"max_floor,omitempty"`
	SecondaryOrderID string           `json:"secondary_order_id,omitempty"`
	StopLimitPrice   fixedpoint.Value `json:"stop_limit_price,omitempty"`
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

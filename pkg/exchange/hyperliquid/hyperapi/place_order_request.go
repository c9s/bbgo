package hyperapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Response.Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Response.Data

type OrderResponse struct {
	Statuses []struct {
		Resting *struct {
			Oid int `json:"oid"`
		} `json:"resting,omitempty"`
		Error  string `json:"error,omitempty"`
		Filled *struct {
			TotalSz fixedpoint.Value `json:"totalSz"`
			AvgPx   fixedpoint.Value `json:"avgPx"`
			Oid     int              `json:"oid"`
		} `json:"filled,omitempty"`
	} `json:"statuses"`
}

type Order struct {
	Asset         string    `json:"a"`
	IsBuy         bool      `json:"b"`
	Size          string    `json:"s"`
	Price         string    `json:"p"`
	ReduceOnly    bool      `json:"r"`
	ClientOrderID *string   `json:"c"`
	OrderType     OrderType `json:"t"`
}

type OrderType struct {
	Limit   LimitOrderType   `json:"limit,omitempty"`
	Trigger TriggerOrderType `json:"trigger,omitempty"`
}

type LimitOrderType struct {
	Tif TimeInForce `json:"tif" validate:"Alo,Ioc,Gtc"`
}

type TriggerOrderType struct {
	IsMarket  bool   `json:"isMarket"`
	TriggerPx string `json:"triggerPx,omitempty"`
	Tpsl      string `json:"tpsl,omitempty" validate:"tp,sl"`
}

//go:generate PostRequest -url "/exchange" -type PlaceOrderRequest -responseDataType OrderResponse
type PlaceOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	metaType ReqTypeInfo `param:"type" default:"order" validValues:"order"`

	orders []Order `param:"orders,required"`

	grouping string `param:"grouping" default:"na" validValues:"na,normalTpsl,positionTpsl"`

	builder *struct {
		FeeAddress string `json:"b"`
		FeeSize    string `json:"f"`
	} `param:"builder,omitempty"`
}

func (c *Client) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{
		client:   c,
		metaType: ReqSubmitOrder,
	}
}

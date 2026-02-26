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
	Asset         int       `json:"a"`
	IsBuy         bool      `json:"b"`
	Price         string    `json:"p"`
	Size          string    `json:"s"`
	ReduceOnly    bool      `json:"r"`
	OrderType     OrderType `json:"t"`
	ClientOrderID *string   `json:"c,omitempty"`
}

type OrderType struct {
	Limit   *LimitOrderType   `json:"limit,omitempty"`
	Trigger *TriggerOrderType `json:"trigger,omitempty"`
}

type LimitOrderType struct {
	Tif TimeInForce `json:"tif,omitempty" validate:"Alo,Ioc,Gtc"`
}

type TriggerOrderType struct {
	IsMarket  bool   `json:"isMarket"`
	TriggerPx string `json:"triggerPx,omitempty"`
	Tpsl      Tpsl   `json:"tpsl,omitempty" validate:"tp,sl"`
}

type BuilderInfo struct {
	Builder string `json:"b"`
	Fee     int    `json:"f"`
}

//go:generate PostRequest -url "/exchange" -type PlaceOrderRequest -responseDataType OrderResponse
type PlaceOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	metaType ReqTypeInfo `param:"type" default:"order" validValues:"order"`

	orders []Order `param:"orders,required"`

	grouping Grouping `param:"grouping" default:"na" validValues:"na,normalTpsl,positionTpsl"`

	builder *BuilderInfo `param:"builder,omitempty"`
}

func (c *Client) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{
		client:   c,
		metaType: ReqSubmitOrder,
	}
}

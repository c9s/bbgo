package hyperapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Response.Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Response.Data

type OrderResponse struct {
	Statuses []struct {
		Resting struct {
			Oid   string `json:"oid"`
			Error string `json:"error"`
		} `json:"resting,omitempty"`
		Filled struct {
			TotalSz string `json:"totalSz"`
			AvgPx   string `json:"avgPx"`
			Oid     int64  `json:"oid"`
		} `json:"filled,omitempty"`
	} `json:"statuses"`
}

type Order struct {
	Asset         string  `json:"a"`
	IsBuy         bool    `json:"b"`
	Size          string  `json:"s"`
	Price         string  `json:"p"`
	ReduceOnly    bool    `json:"r"`
	ClientOrderID *string `json:"c"`
	OrderType     struct {
		Limit struct {
			Tif TimeInForce `json:"tif" validate:"Alo,Ioc,Gtc"`
		} `json:"limit"`
		Trigger struct {
			IsMarket  bool   `json:"isMarket"`
			TriggerPx string `json:"triggerPx"`
			Tpsl      string `json:"tpsl" validate:"tp,sl"`
		}
	} `json:"t"`
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

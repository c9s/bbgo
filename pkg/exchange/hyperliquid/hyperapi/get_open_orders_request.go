package hyperapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Response.Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Response.Data

type OpenOrder struct {
	Coin             string                     `json:"coin"`
	IsPositionTpsl   bool                       `json:"isPositionTpsl"`
	IsTrigger        bool                       `json:"isTrigger"`
	LimitPx          fixedpoint.Value           `json:"limitPx"`
	Oid              int64                      `json:"oid"`
	OrderType        string                     `json:"orderType"`
	OrigSz           fixedpoint.Value           `json:"origSz"`
	ReduceOnly       bool                       `json:"reduceOnly"`
	Side             string                     `json:"side"`
	Sz               fixedpoint.Value           `json:"sz"`
	Timestamp        types.MillisecondTimestamp `json:"timestamp"`
	TriggerCondition string                     `json:"triggerCondition"`
	TriggerPx        fixedpoint.Value           `json:"triggerPx"`
}

//go:generate PostRequest -url "/info" -type GetOpenOrdersRequest -responseDataType []OpenOrder
type GetOpenOrdersRequest struct {
	client requestgen.APIClient

	user string  `param:"user,required"`
	dex  *string `param:"dex"`

	metaType ReqTypeInfo `param:"type" default:"frontendOpenOrders" validValues:"frontendOpenOrders"`
}

func (c *Client) NewGetOpenOrdersRequest() *GetOpenOrdersRequest {
	return &GetOpenOrdersRequest{
		client:   c,
		metaType: ReqFrontendOpenOrders,
	}
}

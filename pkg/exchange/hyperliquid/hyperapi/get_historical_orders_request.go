package hyperapi

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Response.Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Response.Data

type HistoricalOrder struct {
	Coin             string                     `json:"coin"`
	Side             string                     `json:"side"`
	LimitPx          string                     `json:"limitPx"`
	Sz               string                     `json:"sz"`
	Oid              int64                      `json:"oid"`
	Timestamp        types.MillisecondTimestamp `json:"timestamp"`
	TriggerCondition string                     `json:"triggerCondition"`
	IsTrigger        bool                       `json:"isTrigger"`
	TriggerPx        string                     `json:"triggerPx"`
	Children         []any                      `json:"children"`
	IsPositionTpsl   bool                       `json:"isPositionTpsl"`
	ReduceOnly       bool                       `json:"reduceOnly"`
	OrderType        string                     `json:"orderType"`
	OrigSz           string                     `json:"origSz"`
	Tif              string                     `json:"tif"`
	Cloid            *string                    `json:"cloid"`
}

type HistoricalOrdersResponse struct {
	Order           HistoricalOrder            `json:"order"`
	Status          string                     `json:"status"`
	StatusTimestamp types.MillisecondTimestamp `json:"statusTimestamp"`
}

//go:generate PostRequest -url "/info" -type GetHistoricalOrdersRequest -responseDataType []HistoricalOrdersResponse
type GetHistoricalOrdersRequest struct {
	client requestgen.APIClient

	user     string      `param:"user,required"`
	metaType ReqTypeInfo `param:"type" default:"historicalOrders" validValues:"historicalOrders"`
}

func (c *Client) NewGetHistoricalOrdersRequest() *GetHistoricalOrdersRequest {
	return &GetHistoricalOrdersRequest{
		client:   c,
		metaType: ReqHistoricalOrders,
	}
}

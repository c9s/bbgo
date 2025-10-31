package hyperapi

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Response.Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Response.Data

type Trade struct {
	ClosedPnl     string                     `json:"closedPnl,omitempty"`
	Coin          string                     `json:"coin"`
	Crossed       bool                       `json:"crossed"`
	Dir           string                     `json:"dir,omitempty"`
	Hash          string                     `json:"hash"`
	Oid           int64                      `json:"oid"`
	Px            string                     `json:"px"`
	Side          string                     `json:"side"`
	StartPosition string                     `json:"startPosition,omitempty"`
	Sz            string                     `json:"sz"`
	Time          types.MillisecondTimestamp `json:"time"`
	Fee           string                     `json:"fee,omitempty"`
	FeeToken      string                     `json:"feeToken,omitempty"`
	BuilderFee    string                     `json:"builderFee,omitempty"`
	Tid           int64                      `json:"tid,omitempty"`
}

//go:generate PostRequest -url "/info" -type GetTradesHistoryRequest -responseDataType []Trade
type GetTradesHistoryRequest struct {
	client requestgen.APIClient

	user            string                      `param:"user,required"`
	startTime       types.MillisecondTimestamp  `param:"startTime"`
	endTime         *types.MillisecondTimestamp `param:"endTime"`
	aggregateByTime *bool                       `param:"aggregateByTime"`
	metaType        ReqTypeInfo                 `param:"type" default:"userFills" validValues:"userFills,userFillsByTime"`
}

func (c *Client) NewGetOrderHistoryRequest() *GetTradesHistoryRequest {
	return &GetTradesHistoryRequest{
		client:   c,
		metaType: ReqUserFills,
	}
}

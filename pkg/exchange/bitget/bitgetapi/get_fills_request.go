package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Fill struct {
	AccountId       types.StrInt64             `json:"accountId"`
	Symbol          string                     `json:"symbol"`
	OrderId         types.StrInt64             `json:"orderId"`
	FillId          types.StrInt64             `json:"fillId"`
	OrderType       OrderType                  `json:"orderType"`
	Side            OrderSide                  `json:"side"`
	FillPrice       fixedpoint.Value           `json:"fillPrice"`
	FillQuantity    fixedpoint.Value           `json:"fillQuantity"`
	FillTotalAmount fixedpoint.Value           `json:"fillTotalAmount"`
	CreationTime    types.MillisecondTimestamp `json:"cTime"`
	FeeCurrency     string                     `json:"feeCcy"`
	Fees            fixedpoint.Value           `json:"fees"`
}

//go:generate GetRequest -url "/api/spot/v1/trade/fills" -type GetFillsRequest -responseDataType .ServerTime
type GetFillsRequest struct {
	client requestgen.AuthenticatedAPIClient

	// after - order id
	after *string `param:"after"`

	// before - order id
	before *string `param:"before"`

	limit *string `param:"limit"`
}

func (c *RestClient) NewGetFillsRequest() *GetFillsRequest {
	return &GetFillsRequest{client: c}
}

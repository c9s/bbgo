package hyperapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Response.Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Response.Data

type CancelResponse struct {
	Statuses []any `json:"statuses"`
}

//go:generate PostRequest -url "/exchange" -type CancelOrderRequest -responseDataType CancelResponse
type CancelOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	metaType InfoReqType `param:"type" default:"cancel" validValues:"cancel"`

	cancelOrders []struct {
		Asset   int `json:"a"`
		OrderId int `json:"o"`
	} `param:"cancels,required"`
}

func (c *Client) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{
		client:   c,
		metaType: CancelOrder,
	}
}

package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

type CancelOrder struct {
	// OrderId are always numeric. It's confirmed with official customer service. https://t.me/bitgetOpenapi/24172
	OrderId       types.StrInt64 `json:"orderId"`
	ClientOrderId string         `json:"clientOid"`
}

//go:generate PostRequest -url "/api/v2/spot/trade/cancel-order" -type CancelOrderRequest -responseDataType .CancelOrder
type CancelOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol        string  `param:"symbol"`
	orderId       *string `param:"orderId"`
	clientOrderId *string `param:"clientOid"`
}

func (c *Client) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{client: c.Client}
}

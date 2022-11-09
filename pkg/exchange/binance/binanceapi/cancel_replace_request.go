package binanceapi

import (
	"github.com/adshao/go-binance/v2"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type CancelReplaceSpotOrderData struct {
	CancelResult     string         `json:"cancelResult"`
	NewOrderResult   string         `json:"newOrderResult"`
	NewOrderResponse *binance.Order `json:"newOrderResponse"`
}

type CancelReplaceSpotOrderResponse struct {
	Code int                         `json:"code,omitempty"`
	Msg  string                      `json:"msg,omitempty"`
	Data *CancelReplaceSpotOrderData `json:"data"`
}

//go:generate requestgen -method POST -url "/api/v3/order/cancelReplace" -type CancelReplaceSpotOrderRequest -responseType .CancelReplaceSpotOrderResponse
type CancelReplaceSpotOrderRequest struct {
	client                  requestgen.AuthenticatedAPIClient
	symbol                  string                     `param:"symbol"`
	side                    SideType                   `param:"side"`
	cancelReplaceMode       CancelReplaceModeType      `param:"cancelReplaceMode"`
	timeInForce             string                     `param:"timeInForce"`
	quantity                string                     `param:"quantity"`
	quoteOrderQty           string                     `param:"quoteOrderQty"`
	price                   string                     `param:"price"`
	cancelNewClientOrderId  string                     `param:"cancelNewClientOrderId"`
	cancelOrigClientOrderId string                     `param:"cancelOrigClientOrderId"`
	cancelOrderId           int                        `param:"cancelOrderId"`
	newClientOrderId        string                     `param:"newClientOrderId"`
	strategyId              int                        `param:"strategyId"`
	strategyType            int                        `param:"strategyType"`
	stopPrice               string                     `param:"stopPrice"`
	trailingDelta           int                        `param:"trailingDelta"`
	icebergQty              string                     `param:"icebergQty"`
	newOrderRespType        OrderRespType              `param:"newOrderRespType"`
	recvWindow              int                        `param:"recvWindow"`
	timestamp               types.MillisecondTimestamp `param:"timestamp"`
}

func (c *RestClient) NewCancelReplaceSpotOrderRequest() *CancelReplaceSpotOrderRequest {
	return &CancelReplaceSpotOrderRequest{client: c}
}

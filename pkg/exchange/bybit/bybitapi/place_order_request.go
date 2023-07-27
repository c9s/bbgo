package bybitapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type PlaceOrderResponse struct {
	OrderId     string `json:"orderId"`
	OrderLinkId string `json:"orderLinkId"`
}

//go:generate PostRequest -url "/v5/order/create" -type PlaceOrderRequest -responseDataType .PlaceOrderResponse
type PlaceOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	category    Category    `param:"category" validValues:"spot"`
	symbol      string      `param:"symbol"`
	side        Side        `param:"side" validValues:"Buy,Sell"`
	orderType   OrderType   `param:"orderType" validValues:"Market,Limit"`
	qty         string      `param:"qty"`
	orderLinkId string      `param:"orderLinkId"`
	timeInForce TimeInForce `param:"timeInForce"`

	isLeverage       *bool   `param:"isLeverage"`
	price            *string `param:"price"`
	triggerDirection *int    `param:"triggerDirection"`
	// orderFilter default spot
	orderFilter *string `param:"orderFilter"`
	// triggerPrice when submitting an order, if triggerPrice is set, the order will be automatically converted into a conditional order.
	triggerPrice   *string `param:"triggerPrice"`
	triggerBy      *string `param:"triggerBy"`
	orderIv        *string `param:"orderIv"`
	positionIdx    *string `param:"positionIdx"`
	takeProfit     *string `param:"takeProfit"`
	stopLoss       *string `param:"stopLoss"`
	tpTriggerBy    *string `param:"tpTriggerBy"`
	slTriggerBy    *string `param:"slTriggerBy"`
	reduceOnly     *bool   `param:"reduceOnly"`
	closeOnTrigger *bool   `param:"closeOnTrigger"`
	smpType        *string `param:"smpType"`
	mmp            *bool   `param:"mmp"` // option only
	tpslMode       *string `param:"tpslMode"`
	tpLimitPrice   *string `param:"tpLimitPrice"`
	slLimitPrice   *string `param:"slLimitPrice"`
	tpOrderType    *string `param:"tpOrderType"`
	slOrderType    *string `param:"slOrderType"`
}

func (c *RestClient) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{
		client:   c,
		category: CategorySpot,
	}
}

package bybitapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type CancelOrderResponse struct {
	OrderId     string `json:"orderId"`
	OrderLinkId string `json:"orderLinkId"`
}

//go:generate PostRequest -url "/v5/order/cancel" -type CancelOrderRequest -responseDataType .CancelOrderResponse
type CancelOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	category Category `param:"category" validValues:"spot"`
	symbol   string   `param:"symbol"`
	// User customised order ID. Either orderId or orderLinkId is required
	orderLinkId string `param:"orderLinkId"`

	orderId *string `param:"orderId"`
	// orderFilter default type is Order
	// tpsl order type are not currently supported
	orderFilter *string `param:"timeInForce" validValues:"Order"`
}

func (c *RestClient) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{
		client:   c,
		category: CategorySpot,
	}
}

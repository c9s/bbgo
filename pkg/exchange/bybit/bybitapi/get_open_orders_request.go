package bybitapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type OrdersResponse struct {
	List           []Order `json:"list"`
	NextPageCursor string  `json:"nextPageCursor"`
	Category       string  `json:"category"`
}

type Order struct {
	OrderId     string                     `json:"orderId"`
	OrderLinkId string                     `json:"orderLinkId"`
	Symbol      string                     `json:"symbol"`
	Side        Side                       `json:"side"`
	OrderStatus OrderStatus                `json:"orderStatus"`
	OrderType   OrderType                  `json:"orderType"`
	TimeInForce TimeInForce                `json:"timeInForce"`
	Price       fixedpoint.Value           `json:"price"`
	CreatedTime types.MillisecondTimestamp `json:"createdTime"`
	UpdatedTime types.MillisecondTimestamp `json:"updatedTime"`

	// Qty represents **quote coin** if order is market buy
	Qty fixedpoint.Value `json:"qty"`

	// AvgPrice is supported in both RESTful API and WebSocket.
	//
	// For websocket must notice that:
	// - Normal account is not supported.
	// - For normal account USDT Perp and Inverse derivatives trades, if a partially filled order, and the
	// final orderStatus is Cancelled, then avgPrice is "0"
	AvgPrice fixedpoint.Value `json:"avgPrice"`

	// CumExecQty is supported in both RESTful API and WebSocket.
	CumExecQty fixedpoint.Value `json:"cumExecQty"`

	// CumExecValue is supported in both RESTful API and WebSocket.
	// However, it's **not** supported for **normal accounts** in RESTful API.
	CumExecValue fixedpoint.Value `json:"cumExecValue"`

	// CumExecFee is supported in both RESTful API and WebSocket.
	// However, it's **not** supported for **normal accounts** in RESTful API.
	// For websocket normal spot, it is the execution fee per single fill.
	CumExecFee fixedpoint.Value `json:"cumExecFee"`

	BlockTradeId       string           `json:"blockTradeId"`
	IsLeverage         string           `json:"isLeverage"`
	PositionIdx        int              `json:"positionIdx"`
	CancelType         string           `json:"cancelType"`
	RejectReason       string           `json:"rejectReason"`
	LeavesQty          fixedpoint.Value `json:"leavesQty"`
	LeavesValue        fixedpoint.Value `json:"leavesValue"`
	StopOrderType      string           `json:"stopOrderType"`
	OrderIv            string           `json:"orderIv"`
	TriggerPrice       fixedpoint.Value `json:"triggerPrice"`
	TakeProfit         fixedpoint.Value `json:"takeProfit"`
	StopLoss           fixedpoint.Value `json:"stopLoss"`
	TpTriggerBy        string           `json:"tpTriggerBy"`
	SlTriggerBy        string           `json:"slTriggerBy"`
	TriggerDirection   int              `json:"triggerDirection"`
	TriggerBy          string           `json:"triggerBy"`
	LastPriceOnCreated string           `json:"lastPriceOnCreated"`
	ReduceOnly         bool             `json:"reduceOnly"`
	CloseOnTrigger     bool             `json:"closeOnTrigger"`
	SmpType            string           `json:"smpType"`
	SmpGroup           int              `json:"smpGroup"`
	SmpOrderId         string           `json:"smpOrderId"`
	TpslMode           string           `json:"tpslMode"`
	TpLimitPrice       string           `json:"tpLimitPrice"`
	SlLimitPrice       string           `json:"slLimitPrice"`
	PlaceType          string           `json:"placeType"`
}

//go:generate GetRequest -url "/v5/order/realtime" -type GetOpenOrdersRequest -responseDataType .OrdersResponse
type GetOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	category    Category  `param:"category,query" validValues:"spot"`
	symbol      *string   `param:"symbol,query"`
	baseCoin    *string   `param:"baseCoin,query"`
	settleCoin  *string   `param:"settleCoin,query"`
	orderId     *string   `param:"orderId,query"`
	orderLinkId *string   `param:"orderLinkId,query"`
	openOnly    *OpenOnly `param:"openOnly,query" validValues:"0"`
	orderFilter *string   `param:"orderFilter,query"`
	limit       *uint64   `param:"limit,query"`
	cursor      *string   `param:"cursor,query"`
}

// NewGetOpenOrderRequest queries unfilled or partially filled orders in real-time. To query older order records,
// please use the order history interface.
func (c *RestClient) NewGetOpenOrderRequest() *GetOpenOrdersRequest {
	return &GetOpenOrdersRequest{
		client:   c,
		category: CategorySpot,
	}
}

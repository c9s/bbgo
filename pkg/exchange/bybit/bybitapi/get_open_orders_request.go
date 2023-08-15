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
	OrderId            string                     `json:"orderId"`
	OrderLinkId        string                     `json:"orderLinkId"`
	BlockTradeId       string                     `json:"blockTradeId"`
	Symbol             string                     `json:"symbol"`
	Price              fixedpoint.Value           `json:"price"`
	Qty                fixedpoint.Value           `json:"qty"`
	Side               Side                       `json:"side"`
	IsLeverage         string                     `json:"isLeverage"`
	PositionIdx        int                        `json:"positionIdx"`
	OrderStatus        OrderStatus                `json:"orderStatus"`
	CancelType         string                     `json:"cancelType"`
	RejectReason       string                     `json:"rejectReason"`
	AvgPrice           fixedpoint.Value           `json:"avgPrice"`
	LeavesQty          fixedpoint.Value           `json:"leavesQty"`
	LeavesValue        fixedpoint.Value           `json:"leavesValue"`
	CumExecQty         fixedpoint.Value           `json:"cumExecQty"`
	CumExecValue       fixedpoint.Value           `json:"cumExecValue"`
	CumExecFee         fixedpoint.Value           `json:"cumExecFee"`
	TimeInForce        TimeInForce                `json:"timeInForce"`
	OrderType          OrderType                  `json:"orderType"`
	StopOrderType      string                     `json:"stopOrderType"`
	OrderIv            string                     `json:"orderIv"`
	TriggerPrice       fixedpoint.Value           `json:"triggerPrice"`
	TakeProfit         fixedpoint.Value           `json:"takeProfit"`
	StopLoss           fixedpoint.Value           `json:"stopLoss"`
	TpTriggerBy        string                     `json:"tpTriggerBy"`
	SlTriggerBy        string                     `json:"slTriggerBy"`
	TriggerDirection   int                        `json:"triggerDirection"`
	TriggerBy          string                     `json:"triggerBy"`
	LastPriceOnCreated string                     `json:"lastPriceOnCreated"`
	ReduceOnly         bool                       `json:"reduceOnly"`
	CloseOnTrigger     bool                       `json:"closeOnTrigger"`
	SmpType            string                     `json:"smpType"`
	SmpGroup           int                        `json:"smpGroup"`
	SmpOrderId         string                     `json:"smpOrderId"`
	TpslMode           string                     `json:"tpslMode"`
	TpLimitPrice       string                     `json:"tpLimitPrice"`
	SlLimitPrice       string                     `json:"slLimitPrice"`
	PlaceType          string                     `json:"placeType"`
	CreatedTime        types.MillisecondTimestamp `json:"createdTime"`
	UpdatedTime        types.MillisecondTimestamp `json:"updatedTime"`
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

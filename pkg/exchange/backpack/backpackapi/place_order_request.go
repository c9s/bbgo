package backpackapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command PostRequest requestgen -method POST

// Order is a resting or just-submitted order.
//
// The API documents two response schemas discriminated on orderType (MarketOrder and
// LimitOrder); this is the union of both, with the fields that only one of them carries left
// optional.
type Order struct {
	Id     string      `json:"id"`
	Symbol string      `json:"symbol"`
	Side   Side        `json:"side"`
	Status OrderStatus `json:"status"`

	OrderType OrderType `json:"orderType"`

	Price    fixedpoint.Value `json:"price,omitempty"`
	Quantity fixedpoint.Value `json:"quantity,omitempty"`

	// QuoteQuantity is only set on reverse market orders.
	QuoteQuantity fixedpoint.Value `json:"quoteQuantity,omitempty"`

	ExecutedQuantity      fixedpoint.Value `json:"executedQuantity"`
	ExecutedQuoteQuantity fixedpoint.Value `json:"executedQuoteQuantity"`

	TimeInForce         TimeInForce         `json:"timeInForce"`
	SelfTradePrevention SelfTradePrevention `json:"selfTradePrevention"`

	PostOnly   bool `json:"postOnly,omitempty"`
	ReduceOnly bool `json:"reduceOnly,omitempty"`

	// ClientId is the numeric custom order id.
	ClientId uint32 `json:"clientId,omitempty"`

	CreatedAt types.MillisecondTimestamp `json:"createdAt"`

	// conditional order fields
	TriggerPrice    fixedpoint.Value             `json:"triggerPrice,omitempty"`
	TriggerQuantity fixedpoint.Value             `json:"triggerQuantity,omitempty"`
	TriggerBy       TriggerBy                    `json:"triggerBy,omitempty"`
	TriggeredAt     NullableMillisecondTimestamp `json:"triggeredAt,omitempty"`

	StopLossTriggerPrice fixedpoint.Value `json:"stopLossTriggerPrice,omitempty"`
	StopLossLimitPrice   fixedpoint.Value `json:"stopLossLimitPrice,omitempty"`
	StopLossTriggerBy    TriggerBy        `json:"stopLossTriggerBy,omitempty"`

	TakeProfitTriggerPrice fixedpoint.Value `json:"takeProfitTriggerPrice,omitempty"`
	TakeProfitLimitPrice   fixedpoint.Value `json:"takeProfitLimitPrice,omitempty"`
	TakeProfitTriggerBy    TriggerBy        `json:"takeProfitTriggerBy,omitempty"`

	SlippageTolerance     fixedpoint.Value      `json:"slippageTolerance,omitempty"`
	SlippageToleranceType SlippageToleranceType `json:"slippageToleranceType,omitempty"`

	RelatedOrderId string `json:"relatedOrderId,omitempty"`
	StrategyId     string `json:"strategyId,omitempty"`
}

// PlaceOrderRequest submits a single order.
//
// Market orders must specify either quantity or quoteQuantity; every other order type must
// specify quantity.
//
//go:generate PostRequest -url "/api/v1/order" -type PlaceOrderRequest -responseType .Order
type PlaceOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol    string    `param:"symbol,required"`
	side      Side      `param:"side,required"`
	orderType OrderType `param:"orderType,required"`

	quantity *string `param:"quantity"`

	// quoteQuantity is the maximum amount of the quote asset to spend (Bid) or receive (Ask),
	// only applicable to market orders.
	quoteQuantity *string `param:"quoteQuantity"`

	price *string `param:"price"`

	timeInForce *TimeInForce `param:"timeInForce"`
	postOnly    *bool        `param:"postOnly"`
	reduceOnly  *bool        `param:"reduceOnly"`

	// clientId is a numeric custom order id.
	clientId *uint32 `param:"clientId"`

	selfTradePrevention *SelfTradePrevention `param:"selfTradePrevention"`

	autoLend        *bool `param:"autoLend"`
	autoLendRedeem  *bool `param:"autoLendRedeem"`
	autoBorrow      *bool `param:"autoBorrow"`
	autoBorrowRepay *bool `param:"autoBorrowRepay"`

	triggerPrice    *string    `param:"triggerPrice"`
	triggerQuantity *string    `param:"triggerQuantity"`
	triggerBy       *TriggerBy `param:"triggerBy"`

	stopLossTriggerPrice *string    `param:"stopLossTriggerPrice"`
	stopLossLimitPrice   *string    `param:"stopLossLimitPrice"`
	stopLossTriggerBy    *TriggerBy `param:"stopLossTriggerBy"`

	takeProfitTriggerPrice *string    `param:"takeProfitTriggerPrice"`
	takeProfitLimitPrice   *string    `param:"takeProfitLimitPrice"`
	takeProfitTriggerBy    *TriggerBy `param:"takeProfitTriggerBy"`

	slippageTolerance     *string                `param:"slippageTolerance"`
	slippageToleranceType *SlippageToleranceType `param:"slippageToleranceType"`
}

func (c *RestClient) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{client: c.withInstruction(InstructionOrderExecute)}
}

package backpackapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET

// OrderHistoryEntry is a historical order.
//
// It is a separate type from Order because the history endpoint encodes createdAt as a naive
// datetime string rather than a unix millisecond timestamp.
type OrderHistoryEntry struct {
	Id     string      `json:"id"`
	Symbol string      `json:"symbol"`
	Side   Side        `json:"side"`
	Status OrderStatus `json:"status"`

	OrderType OrderType `json:"orderType"`

	// CreatedAt is a UTC datetime without a timezone suffix,
	// e.g. "2025-01-21T06:34:54.691858".
	CreatedAt types.Time `json:"createdAt"`

	Price         fixedpoint.Value `json:"price,omitempty"`
	Quantity      fixedpoint.Value `json:"quantity,omitempty"`
	QuoteQuantity fixedpoint.Value `json:"quoteQuantity,omitempty"`

	ExecutedQuantity      fixedpoint.Value `json:"executedQuantity,omitempty"`
	ExecutedQuoteQuantity fixedpoint.Value `json:"executedQuoteQuantity,omitempty"`

	TimeInForce         TimeInForce         `json:"timeInForce"`
	SelfTradePrevention SelfTradePrevention `json:"selfTradePrevention"`

	PostOnly bool `json:"postOnly,omitempty"`

	ClientId uint32 `json:"clientId,omitempty"`

	ExpiryReason    string          `json:"expiryReason,omitempty"`
	SystemOrderType SystemOrderType `json:"systemOrderType,omitempty"`
	StrategyId      string          `json:"strategyId,omitempty"`

	TriggerPrice    fixedpoint.Value `json:"triggerPrice,omitempty"`
	TriggerQuantity fixedpoint.Value `json:"triggerQuantity,omitempty"`
	TriggerBy       TriggerBy        `json:"triggerBy,omitempty"`

	StopLossTriggerPrice fixedpoint.Value `json:"stopLossTriggerPrice,omitempty"`
	StopLossLimitPrice   fixedpoint.Value `json:"stopLossLimitPrice,omitempty"`
	StopLossTriggerBy    TriggerBy        `json:"stopLossTriggerBy,omitempty"`

	TakeProfitTriggerPrice fixedpoint.Value `json:"takeProfitTriggerPrice,omitempty"`
	TakeProfitLimitPrice   fixedpoint.Value `json:"takeProfitLimitPrice,omitempty"`
	TakeProfitTriggerBy    TriggerBy        `json:"takeProfitTriggerBy,omitempty"`

	SlippageTolerance     fixedpoint.Value      `json:"slippageTolerance,omitempty"`
	SlippageToleranceType SlippageToleranceType `json:"slippageToleranceType,omitempty"`
}

//go:generate GetRequest -url "/wapi/v1/history/orders" -type GetOrderHistoryRequest -responseType []OrderHistoryEntry -rateLimiter 1+30/60s
type GetOrderHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol     *string     `param:"symbol"`
	orderId    *string     `param:"orderId"`
	strategyId *string     `param:"strategyId"`
	marketType *MarketType `param:"marketType"`

	// limit defaults to 100 and is capped at 1000.
	limit  *uint64 `param:"limit"`
	offset *uint64 `param:"offset"`

	sortDirection *SortDirection `param:"sortDirection"`
}

func (c *RestClient) NewGetOrderHistoryRequest() *GetOrderHistoryRequest {
	return &GetOrderHistoryRequest{client: c.withInstruction(InstructionOrderHistoryQueryAll)}
}

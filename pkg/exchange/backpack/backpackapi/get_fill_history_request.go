package backpackapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET

// OrderFill is a single fill of an order.
type OrderFill struct {
	OrderId string `json:"orderId"`
	Symbol  string `json:"symbol"`
	Side    Side   `json:"side"`

	Price    fixedpoint.Value `json:"price"`
	Quantity fixedpoint.Value `json:"quantity"`

	Fee       fixedpoint.Value `json:"fee"`
	FeeSymbol string           `json:"feeSymbol"`

	IsMaker bool `json:"isMaker"`

	// Timestamp is a UTC datetime without a timezone suffix.
	Timestamp types.Time `json:"timestamp"`

	TradeId int64 `json:"tradeId,omitempty"`

	// ClientId is a string on this endpoint, unlike the uint32 used on the order endpoints.
	ClientId string `json:"clientId,omitempty"`

	SystemOrderType SystemOrderType `json:"systemOrderType,omitempty"`
}

// GetFillHistoryRequest queries the fill history of the account.
//
// Since 2025-04-22 this returns fills from system orders as well; pass FillTypeUser to
// restrict the result to the fills of client orders.
//
//go:generate GetRequest -url "/wapi/v1/history/fills" -type GetFillHistoryRequest -responseType []OrderFill -rateLimiter 1+30/60s
type GetFillHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol     *string     `param:"symbol"`
	orderId    *string     `param:"orderId"`
	strategyId *string     `param:"strategyId"`
	marketType *MarketType `param:"marketType"`

	// from is inclusive and to is exclusive.
	from *time.Time `param:"from,milliseconds"`
	to   *time.Time `param:"to,milliseconds"`

	fillType *FillType `param:"fillType"`

	// limit defaults to 100 and is capped at 1000.
	limit  *uint64 `param:"limit"`
	offset *uint64 `param:"offset"`

	sortDirection *SortDirection `param:"sortDirection"`
}

func (c *RestClient) NewGetFillHistoryRequest() *GetFillHistoryRequest {
	return &GetFillHistoryRequest{client: c.withInstruction(InstructionFillHistoryQueryAll)}
}

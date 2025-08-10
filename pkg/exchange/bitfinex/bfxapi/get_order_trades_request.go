package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// OrderTradeDetail represents a trade detail for a Bitfinex order trade API response.
// It matches the array response format from Bitfinex.
type OrderTradeDetail struct {
	TradeID    int64                      // trade ID
	Symbol     string                     // trading pair symbol
	Time       types.MillisecondTimestamp // timestamp in ms
	OrderID    int64                      // order ID
	ExecAmount fixedpoint.Value           // executed amount
	ExecPrice  fixedpoint.Value           // executed price

	_ any
	_ any

	Maker         int              // maker flag
	Fee           fixedpoint.Value // fee amount
	FeeCurrency   string           // fee currency
	ClientOrderID int64            // client order ID
}

// UnmarshalJSON parses the Bitfinex order trades API array response into OrderTradeDetail fields.
func (t *OrderTradeDetail) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, t, 0)
}

// GetOrderTradesRequest represents a request for Bitfinex order trades API.
// API: https://docs.bitfinex.com/reference/rest-auth-order-trades
//
//go:generate requestgen -type GetOrderTradesRequest -method POST -url "/v2/auth/r/order/:symbol::id/trades" -responseType []OrderTradeDetail
type GetOrderTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol,slug,required"` // trading pair symbol (e.g., "tETHUSD")
	id     int64  `param:"id,slug,required"`     // order ID
}

// NewGetOrderTradesRequest creates a new GetOrderTradesRequest instance.
func (c *Client) NewGetOrderTradesRequest() *GetOrderTradesRequest {
	return &GetOrderTradesRequest{client: c}
}

package bfxapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	"github.com/c9s/requestgen"
)

// API: https://docs.bitfinex.com/reference/rest-auth-submit-order
//go:generate requestgen -type SubmitOrderRequest -method POST -url "/v2/auth/w/order/submit" -responseType .SubmitOrderResponse

// SubmitOrderRequest represents a Bitfinex order submission request.
type SubmitOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol    string `param:"symbol"`
	amount    string `param:"amount"`
	price     string `param:"price"`
	orderType string `param:"type"`

	groupId       int64     `param:"gid,omitempty"`
	clientOrderId int64     `param:"cid,omitempty"`
	flags         OrderFlag `param:"flags,omitempty"`
}

// NewSubmitOrderRequest creates a new SubmitOrderRequest.
func (c *Client) NewSubmitOrderRequest() *SubmitOrderRequest {
	return &SubmitOrderRequest{client: c}
}

// SubmitOrderResponse represents the response from Bitfinex order submission.
type SubmitOrderResponse struct {
	mts       types.MillisecondTimestamp
	typeStr   string
	messageID *string
	_         any // unused field
	data      []OrderData
	code      *int64
	status    string
	text      string
}

// OrderData represents a single order in the response DATA array.
type OrderData struct {
	Id            int64
	GroupOrderID  *int64
	ClientOrderID int64
	Symbol        string
	CreatedAt     types.MillisecondTimestamp
	UpdatedAt     types.MillisecondTimestamp
	Amount        fixedpoint.Value
	AmountOrig    fixedpoint.Value

	OrderType string
	TypePrev  *string

	// MtsTif - Millisecond epoch timestamp for TIF (Time-In-Force)
	MtsTif        *int64
	_             any
	Flags         OrderFlag
	Status        OrderStatus
	_             any
	_             any
	Price         fixedpoint.Value
	PriceAvg      fixedpoint.Value
	PriceTrailing float64
	PriceAuxLimit float64
	_3            interface{}
	_4            interface{}
	_5            interface{}
	Notify        int64
	Hidden        int64
	PlacedID      *int64
	_6            interface{}
	_7            interface{}
	Routing       string
	_8            interface{}
	_9            interface{}
	Meta          interface{}
}

// UnmarshalJSON parses the Bitfinex SubmitOrderResponse JSON array.
func (r *SubmitOrderResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

// UnmarshalJSON parses the Bitfinex OrderData JSON array.
func (o *OrderData) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, o, 0)
}

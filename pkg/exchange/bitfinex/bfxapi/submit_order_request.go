package bfxapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	"github.com/c9s/requestgen"
)

// API: https://docs.bitfinex.com/reference/rest-auth-submit-order
//go:generate requestgen -type SubmitOrderRequest -method POST -url "/v2/auth/w/order/submit" -responseType .SubmitOrderResponse

// SubmitOrderRequest represents a Bitfinex order submission request.
type SubmitOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol    string    `param:"symbol,required"`
	amount    string    `param:"amount,required"`
	price     *string   `param:"price"`
	orderType OrderType `param:"type" default:"EXCHANGE LIMIT"`

	groupId       *int64     `param:"gid,omitempty"`
	clientOrderId *int64     `param:"cid,omitempty"`
	flags         *OrderFlag `param:"flags,omitempty"`
}

// NewSubmitOrderRequest creates a new SubmitOrderRequest.
func (c *Client) NewSubmitOrderRequest() *SubmitOrderRequest {
	return &SubmitOrderRequest{client: c}
}

// SubmitOrderResponse represents the response from Bitfinex order submission.
type SubmitOrderResponse struct {
	Time types.MillisecondTimestamp
	Type string // Notification's type ("on-req")

	MessageID *int // Unique notification's ID

	_      any // unused field
	Data   []OrderData
	Code   *int64 // W.I.P. (work in progress)
	Status string
	Text   string // Additional notification description
}

// OrderData represents a single order in the response DATA array.
type OrderData struct {
	OrderID       int64
	GroupOrderID  *int64
	ClientOrderID *int64
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
	PriceTrailing fixedpoint.Value
	PriceAuxLimit fixedpoint.Value
	_             any
	_             any
	_             any
	Notify        int64
	Hidden        int64
	PlacedID      *int64
	_             any
	_             any
	Routing       string
	_             any
	_             any
	Meta          json.RawMessage
}

// UnmarshalJSON parses the Bitfinex SubmitOrderResponse JSON array.
func (r *SubmitOrderResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

// UnmarshalJSON parses the Bitfinex OrderData JSON array.
func (o *OrderData) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, o, 0)
}

// String returns a human readable summary of the order data, skipping nil or empty fields.
func (o *OrderData) String() string {
	var buf []string

	buf = append(buf, fmt.Sprintf("#%d %s %s %s", o.OrderID, o.Symbol, o.OrderType, o.Status))

	if o.GroupOrderID != nil {
		buf = append(buf, fmt.Sprintf("GroupOrderID:%d", *o.GroupOrderID))
	}
	if o.ClientOrderID != nil {
		buf = append(buf, fmt.Sprintf("ClientOrderID:%d", *o.ClientOrderID))
	}

	if !o.Price.IsZero() {
		buf = append(buf, fmt.Sprintf("Price=%s", o.Price.String()))
	}

	if !o.PriceAvg.IsZero() {
		buf = append(buf, fmt.Sprintf("PriceAvg=%s", o.PriceAvg.String()))
	}

	if !o.Amount.IsZero() {
		buf = append(buf, fmt.Sprintf("Amount=%s", o.Amount.String()))
	}

	if !o.AmountOrig.IsZero() {
		buf = append(buf, fmt.Sprintf("AmountOrig=%s", o.AmountOrig.String()))
	}

	if !time.Time(o.CreatedAt).IsZero() {
		buf = append(buf, fmt.Sprintf("CreatedAt=%s", o.CreatedAt))
	}

	if !time.Time(o.UpdatedAt).IsZero() {
		buf = append(buf, fmt.Sprintf("UpdatedAt=%s", o.UpdatedAt))
	}

	if o.TypePrev != nil && *o.TypePrev != "" {
		buf = append(buf, fmt.Sprintf("TypePrev=%s", *o.TypePrev))
	}
	if o.MtsTif != nil {
		buf = append(buf, fmt.Sprintf("MtsTif=%d", *o.MtsTif))
	}
	if o.Flags != 0 {
		buf = append(buf, fmt.Sprintf("Flags=%d", o.Flags))
	}
	if !o.PriceTrailing.IsZero() {
		buf = append(buf, fmt.Sprintf("PriceTrailing=%s", o.PriceTrailing.String()))
	}
	if !o.PriceAuxLimit.IsZero() {
		buf = append(buf, fmt.Sprintf("PriceAuxLimit=%s", o.PriceAuxLimit.String()))
	}
	if o.Notify != 0 {
		buf = append(buf, fmt.Sprintf("Notify=%d", o.Notify))
	}
	if o.Hidden != 0 {
		buf = append(buf, fmt.Sprintf("Hidden=%d", o.Hidden))
	}
	if o.PlacedID != nil {
		buf = append(buf, fmt.Sprintf("PlacedID=%d", *o.PlacedID))
	}
	if o.Routing != "" {
		buf = append(buf, fmt.Sprintf("Routing=%s", o.Routing))
	}
	if o.Meta != nil && len(o.Meta) > 0 {
		buf = append(buf, fmt.Sprintf("Meta=%s", string(o.Meta)))
	}

	return "Order[ " + strings.Join(buf, ", ") + "]"
}

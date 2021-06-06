package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/util"
)

func init() {
	// make sure we can cast Order to PlainText
	_ = PlainText(Order{})
	_ = PlainText(&Order{})
}

// MarginOrderSideEffectType define side effect type for orders
type MarginOrderSideEffectType string

var (
	SideEffectTypeNoSideEffect MarginOrderSideEffectType = "NO_SIDE_EFFECT"
	SideEffectTypeMarginBuy    MarginOrderSideEffectType = "MARGIN_BUY"
	SideEffectTypeAutoRepay    MarginOrderSideEffectType = "AUTO_REPAY"
)

func (t *MarginOrderSideEffectType) UnmarshalJSON(data []byte) error {
	var s string
	var err = json.Unmarshal(data, &s)
	if err != nil {
		return errors.Wrapf(err, "unable to unmarshal side effect type: %s", data)
	}

	switch strings.ToUpper(s) {

	case string(SideEffectTypeNoSideEffect), "":
		*t = SideEffectTypeNoSideEffect
		return nil

	case string(SideEffectTypeMarginBuy), "BORROW", "MARGINBUY":
		*t = SideEffectTypeMarginBuy
		return nil

	case string(SideEffectTypeAutoRepay), "REPAY", "AUTOREPAY":
		*t = SideEffectTypeAutoRepay
		return nil

	}

	return fmt.Errorf("invalid side effect type: %s", data)
}

// OrderType define order type
type OrderType string

const (
	OrderTypeLimit      OrderType = "LIMIT"
	OrderTypeLimitMaker OrderType = "LIMIT_MAKER"
	OrderTypeMarket     OrderType = "MARKET"
	OrderTypeStopLimit  OrderType = "STOP_LIMIT"
	OrderTypeStopMarket OrderType = "STOP_MARKET"
	OrderTypeIOCLimit   OrderType = "IOC_LIMIT"
)

/*
func (t *OrderType) Scan(v interface{}) error {
	switch d := v.(type) {
	case string:
		*t = OrderType(d)

	default:
		return errors.New("order type scan error, type unsupported")

	}
	return nil
}
*/

const NoClientOrderID = "0"

type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"
	OrderStatusRejected        OrderStatus = "REJECTED"
)

type SubmitOrder struct {
	ClientOrderID string `json:"clientOrderID" db:"client_order_id"`

	Symbol string    `json:"symbol" db:"symbol"`
	Side   SideType  `json:"side" db:"side"`
	Type   OrderType `json:"orderType" db:"order_type"`

	Quantity  float64 `json:"quantity" db:"quantity"`
	Price     float64 `json:"price" db:"price"`
	StopPrice float64 `json:"stopPrice,omitempty" db:"stop_price"`

	Market Market `json:"-" db:"-"`

	// TODO: we can probably remove these field
	StopPriceString string `json:"-"`
	PriceString     string `json:"-"`
	QuantityString  string `json:"-"`

	TimeInForce string `json:"timeInForce,omitempty" db:"time_in_force"` // GTC, IOC, FOK

	GroupID uint32 `json:"groupID,omitempty"`

	MarginSideEffect MarginOrderSideEffectType `json:"marginSideEffect,omitempty"` // AUTO_REPAY = repay, MARGIN_BUY = borrow, defaults to  NO_SIDE_EFFECT
}

func (o *SubmitOrder) String() string {
	return fmt.Sprintf("SubmitOrder %s %s %s %f @ %f", o.Symbol, o.Type, o.Side, o.Quantity, o.Price)
}

func (o *SubmitOrder) PlainText() string {
	return fmt.Sprintf("SubmitOrder %s %s %s %f @ %f", o.Symbol, o.Type, o.Side, o.Quantity, o.Price)
}

func (o *SubmitOrder) SlackAttachment() slack.Attachment {
	var fields = []slack.AttachmentField{
		{Title: "Symbol", Value: o.Symbol, Short: true},
		{Title: "Side", Value: string(o.Side), Short: true},
		{Title: "Quantity", Value: o.QuantityString, Short: true},
	}

	if len(o.PriceString) > 0 {
		fields = append(fields, slack.AttachmentField{Title: "Price", Value: o.PriceString, Short: true})
	}

	return slack.Attachment{
		Color: SideToColorName(o.Side),
		Title: string(o.Type) + " Order " + string(o.Side),
		// Text:   "",
		Fields: fields,
	}
}

type Order struct {
	SubmitOrder

	Exchange         ExchangeName `json:"exchange" db:"exchange"`
	GID              uint64       `json:"gid" db:"gid"`
	OrderID          uint64       `json:"orderID" db:"order_id"` // order id
	Status           OrderStatus  `json:"status" db:"status"`
	ExecutedQuantity float64      `json:"executedQuantity" db:"executed_quantity"`
	IsWorking        bool         `json:"isWorking" db:"is_working"`
	CreationTime     Time         `json:"creationTime" db:"created_at"`
	UpdateTime       Time         `json:"updateTime" db:"updated_at"`

	IsMargin   bool `json:"isMargin" db:"is_margin"`
	IsIsolated bool `json:"isIsolated" db:"is_isolated"`
}

// Backup backs up the current order quantity to a SubmitOrder object
// so that we can post the order later when we want to restore the orders.
func (o Order) Backup() SubmitOrder {
	so := o.SubmitOrder
	so.Quantity = o.Quantity - o.ExecutedQuantity

	// ClientOrderID can not be reused
	so.ClientOrderID = ""
	return so
}

func (o Order) String() string {
	return fmt.Sprintf("ORDER %s %d %s %s %f/%f @ %f -> %s", o.Exchange.String(), o.OrderID, o.Symbol, o.Side, o.ExecutedQuantity, o.Quantity, o.Price, o.Status)
}

// PlainText is used for telegram-styled messages
func (o Order) PlainText() string {
	return fmt.Sprintf("Order %s %s %s %s @ %s %s/%s -> %s",
		o.Exchange.String(),
		o.Symbol,
		o.Type,
		o.Side,
		util.FormatFloat(o.Price, 2),
		util.FormatFloat(o.ExecutedQuantity, 2),
		util.FormatFloat(o.Quantity, 4),
		o.Status)
}

func (o Order) SlackAttachment() slack.Attachment {
	var fields = []slack.AttachmentField{
		{Title: "Exchange", Value: o.Exchange.String(), Short: true},
		{Title: "Symbol", Value: o.Symbol, Short: true},
		{Title: "Side", Value: string(o.Side), Short: true},
		{Title: "Quantity", Value: o.QuantityString, Short: true},
		{Title: "Executed Quantity", Value: util.FormatFloat(o.ExecutedQuantity, 4), Short: true},
	}

	if len(o.PriceString) > 0 {
		fields = append(fields, slack.AttachmentField{Title: "Price", Value: o.PriceString, Short: true})
	}

	fields = append(fields, slack.AttachmentField{
		Title: "Order ID", Value: strconv.FormatUint(o.OrderID, 10), Short: true,
	})

	return slack.Attachment{
		Color: SideToColorName(o.Side),
		Title: string(o.Type) + " Order " + string(o.Side),
		// Text:   "",
		Fields: fields,
	}
}

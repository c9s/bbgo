package types

import (
	"time"

	"github.com/slack-go/slack"
)

// OrderType define order type
type OrderType string

const (
	OrderTypeLimit     OrderType = "LIMIT"
	OrderTypeMarket    OrderType = "MARKET"
	OrderTypeStopLimit OrderType = "STOP_LIMIT"
	OrderTypeStopMarket    OrderType = "STOP_MARKET"
)

type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"
	OrderStatusRejected        OrderStatus = "REJECTED"
)

type Order struct {
	SubmitOrder

	OrderID          uint64       `json:"orderID"` // order id
	Status           OrderStatus `json:"status"`
	ExecutedQuantity float64     `json:"executedQuantity"`
	CreationTime time.Time
}

type SubmitOrder struct {
	ClientOrderID string `json:"clientOrderID"`

	Symbol string
	Side   SideType
	Type   OrderType

	Quantity float64
	Price    float64
	StopPrice float64

	Market Market

	// TODO: we can probably remove these field
	StopPriceString string
	PriceString    string
	QuantityString string

	TimeInForce string `json:"timeInForce"` // GTC, IOC, FOK
}

func (o *SubmitOrder) SlackAttachment() slack.Attachment {
	var fields = []slack.AttachmentField{
		{Title: "Symbol", Value: o.Symbol, Short: true},
		{Title: "Side", Value: string(o.Side), Short: true},
		{Title: "Volume", Value: o.QuantityString, Short: true},
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

package types

import (
	"time"

	"github.com/slack-go/slack"
)

// OrderType define order type
type OrderType string

const (
	OrderTypeLimit      OrderType = "LIMIT"
	OrderTypeMarket     OrderType = "MARKET"
	OrderTypeStopLimit  OrderType = "STOP_LIMIT"
	OrderTypeStopMarket OrderType = "STOP_MARKET"
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

	OrderID          uint64      `json:"orderID" db:"order_id"` // order id
	Status           OrderStatus `json:"status" db:"status"`
	ExecutedQuantity float64     `json:"executedQuantity" db:"executed_quantity"`
	IsWorking        bool        `json:"isWorking" db:"is_working"`
	CreationTime     time.Time   `json:"creationTime" db:"created_at"`
}

type SubmitOrder struct {
	ClientOrderID string `json:"clientOrderID" db:"client_order_id"`

	Symbol string    `json:"symbol" db:"symbol"`
	Side   SideType  `json:"side" db:"side"`
	Type   OrderType `json:"orderType" db:"order_type"`

	Quantity  float64 `json:"quantity" db:"quantity"`
	Price     float64 `json:"price" db:"price"`
	StopPrice float64 `json:"stopPrice" db:"stop_price"`

	Market Market `json:"market" db:"-"`

	// TODO: we can probably remove these field
	StopPriceString string `json:"-"`
	PriceString     string `json:"-"`
	QuantityString  string `json:"-"`

	TimeInForce string `json:"timeInForce" db:"time_in_force"` // GTC, IOC, FOK
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

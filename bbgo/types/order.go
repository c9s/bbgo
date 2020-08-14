package types

import (
	"github.com/adshao/go-binance"
	"github.com/slack-go/slack"
)

// OrderType define order type
type OrderType string

const (
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeMarket OrderType = "MARKET"
)

type SubmitOrder struct {
	Symbol   string
	Side     SideType
	Type     OrderType
	Quantity string
	Price    string

	TimeInForce binance.TimeInForceType
}

func (o *SubmitOrder) SlackAttachment() slack.Attachment {
	var fields = []slack.AttachmentField{
		{Title: "Symbol", Value: o.Symbol, Short: true},
		{Title: "Side", Value: string(o.Side), Short: true},
		{Title: "Volume", Value: o.Quantity, Short: true},
	}

	if len(o.Price) > 0 {
		fields = append(fields, slack.AttachmentField{Title: "Price", Value: o.Price, Short: true})
	}

	return slack.Attachment{
		Color: SideToColorName(o.Side),
		Title: string(o.Type) + " Order " + string(o.Side),
		// Text:   "",
		Fields: fields,
	}
}

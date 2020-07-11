package types

import (
	"github.com/adshao/go-binance"
	"github.com/slack-go/slack"
)

type Order struct {
	Symbol    string
	Side      binance.SideType
	Type      binance.OrderType
	VolumeStr string
	PriceStr  string

	TimeInForce binance.TimeInForceType
}

func (o *Order) SlackAttachment() slack.Attachment {
	var fields = []slack.AttachmentField{
		{Title: "Symbol", Value: o.Symbol, Short: true},
		{Title: "Side", Value: string(o.Side), Short: true},
		{Title: "Volume", Value: o.VolumeStr, Short: true},
	}

	if len(o.PriceStr) > 0 {
		fields = append(fields, slack.AttachmentField{Title: "Price", Value: o.PriceStr, Short: true})
	}

	return slack.Attachment{
		Color: SideToColorName(o.Side),
		Title: string(o.Type) + " Order " + string(o.Side),
		// Text:   "",
		Fields: fields,
	}
}


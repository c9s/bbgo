package bbgo

import (
	"github.com/adshao/go-binance"
	"github.com/slack-go/slack"
)

const Green = "#228B22"
const Red = "#800000"

type Order struct {
	Symbol    string
	Side      binance.SideType
	Type      binance.OrderType
	VolumeStr string
	PriceStr  string

	TimeInForce binance.TimeInForceType
}

func (o *Order) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Color: SideToColorName(o.Side),
		Title: "Market Order " + string(o.Side),
		// Text:   "",
		Fields: []slack.AttachmentField{
			{Title: "Side", Value: string(o.Side), Short: true,},
			{Title: "Volume", Value: o.VolumeStr, Short: true,},
		},
	}
}

func SideToColorName(side binance.SideType) string {
	if side == binance.SideTypeBuy {
		return Green
	}
	if side == binance.SideTypeSell {
		return Red
	}

	return "#f0f0f0"
}



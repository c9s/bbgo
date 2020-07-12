package types

import (
	"github.com/slack-go/slack"
	"time"
)

type Trade struct {
	ID          int64
	Price       float64
	Volume      float64
	Side        string
	IsBuyer     bool
	IsMaker     bool
	Time        time.Time
	Symbol      string
	Fee         float64
	FeeCurrency string
}

func (trade Trade) SlackAttachment() slack.Attachment {
	var color = ""
	if trade.IsBuyer {
		color = "#228B22"
	} else {
		color = "#DC143C"
	}

	market, ok := FindMarket(trade.Symbol)
	if !ok {
		return slack.Attachment{
			Title: "New Trade",
			Color: color,
		}
	}

	return slack.Attachment{
		Title: "New Trade",
		Color: color,
		// Pretext:       "",
		// Text:          "",
		Fields: []slack.AttachmentField{
			{Title: "Symbol", Value: trade.Symbol, Short: true,},
			{Title: "Side", Value: trade.Side, Short: true,},
			{Title: "Price", Value: market.FormatPrice(trade.Price), Short: true,},
			{Title: "Volume", Value: market.FormatVolume(trade.Volume), Short: true,},
		},
		// Footer:     tradingCtx.TradeStartTime.Format(time.RFC822),
		// FooterIcon: "",
	}
}

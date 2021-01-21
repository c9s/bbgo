package types

import (
	"fmt"
	"sync"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/util"
)

func init() {
	// make sure we can cast Trade to PlainText
	_ = PlainText(Trade{})
	_ = PlainText(&Trade{})
}

type TradeSlice struct {
	mu    sync.Mutex
	Items []Trade
}

func (s *TradeSlice) Slice() []Trade {
	s.mu.Lock()
	slice := make([]Trade, len(s.Items), len(s.Items))
	copy(slice, s.Items)
	s.mu.Unlock()

	return slice
}

func (s *TradeSlice) Append(t Trade) {
	s.mu.Lock()
	s.Items = append(s.Items, t)
	s.mu.Unlock()
}

type Trade struct {
	// GID is the global ID
	GID int64 `json:"gid" db:"gid"`

	// ID is the source trade ID
	ID            int64   `json:"id" db:"id"`
	OrderID       uint64  `json:"orderID" db:"order_id"`
	Exchange      string  `json:"exchange" db:"exchange"`
	Price         float64 `json:"price" db:"price"`
	Quantity      float64 `json:"quantity" db:"quantity"`
	QuoteQuantity float64 `json:"quoteQuantity" db:"quote_quantity"`
	Symbol        string  `json:"symbol" db:"symbol"`

	Side        SideType  `json:"side" db:"side"`
	IsBuyer     bool      `json:"isBuyer" db:"is_buyer"`
	IsMaker     bool      `json:"isMaker" db:"is_maker"`
	Time        time.Time `json:"tradedAt" db:"traded_at"`
	Fee         float64   `json:"fee" db:"fee"`
	FeeCurrency string    `json:"feeCurrency" db:"fee_currency"`

	IsMargin   bool `json:"isMargin" db:"is_margin"`
	IsIsolated bool `json:"isIsolated" db:"is_isolated"`
}

func (trade Trade) PlainText() string {
	return fmt.Sprintf("%s Trade %s %s price %s, quantity %s, amount %s",
		trade.Exchange,
		trade.Symbol,
		trade.Side,
		util.FormatFloat(trade.Price, 2),
		util.FormatFloat(trade.Quantity, 4),
		util.FormatFloat(trade.QuoteQuantity, 2))
}

func (trade Trade) SlackAttachment() slack.Attachment {
	var color = "#DC143C"

	if trade.IsBuyer {
		color = "#228B22"
	}

	return slack.Attachment{
		Text:  fmt.Sprintf("*%s* Trade %s", trade.Symbol, trade.Side),
		Color: color,
		// Pretext:       "",
		// Text:          "",
		Fields: []slack.AttachmentField{
			{Title: "Exchange", Value: trade.Exchange, Short: true},
			{Title: "Price", Value: util.FormatFloat(trade.Price, 2), Short: true},
			{Title: "Volume", Value: util.FormatFloat(trade.Quantity, 4), Short: true},
			{Title: "Amount", Value: util.FormatFloat(trade.QuoteQuantity, 2)},
			{Title: "Fee", Value: util.FormatFloat(trade.Fee, 4), Short: true},
			{Title: "FeeCurrency", Value: trade.FeeCurrency, Short: true},
		},
		// Footer:     tradingCtx.TradeStartTime.Format(time.RFC822),
		// FooterIcon: "",
	}
}

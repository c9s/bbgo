package types

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/datatype"
	"github.com/c9s/bbgo/pkg/util"
)

func init() {
	// make sure we can cast Trade to PlainText
	_ = PlainText(Trade{})
	_ = PlainText(&Trade{})
}

type TradeSlice struct {
	mu     sync.Mutex
	Trades []Trade
}

func (s *TradeSlice) Copy() []Trade {
	s.mu.Lock()
	slice := make([]Trade, len(s.Trades), len(s.Trades))
	copy(slice, s.Trades)
	s.mu.Unlock()

	return slice
}

func (s *TradeSlice) Reverse() {
	slice := s.Trades
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func (s *TradeSlice) Append(t Trade) {
	s.mu.Lock()
	s.Trades = append(s.Trades, t)
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

	Side        SideType      `json:"side" db:"side"`
	IsBuyer     bool          `json:"isBuyer" db:"is_buyer"`
	IsMaker     bool          `json:"isMaker" db:"is_maker"`
	Time        datatype.Time `json:"tradedAt" db:"traded_at"`
	Fee         float64       `json:"fee" db:"fee"`
	FeeCurrency string        `json:"feeCurrency" db:"fee_currency"`

	IsMargin   bool `json:"isMargin" db:"is_margin"`
	IsIsolated bool `json:"isIsolated" db:"is_isolated"`

	StrategyID sql.NullString  `json:"strategyID" db:"strategy"`
	PnL        sql.NullFloat64 `json:"pnl" db:"pnl"`
}

func (trade Trade) String() string {
	return fmt.Sprintf("TRADE %s %s %s %s @ %s, amount %s",
		trade.Exchange,
		trade.Symbol,
		trade.Side,
		util.FormatFloat(trade.Quantity, 4),
		util.FormatFloat(trade.Price, 3),
		util.FormatFloat(trade.QuoteQuantity, 2))
}

func (trade Trade) PlainText() string {
	return trade.String()
}

var slackTradeTextTemplate = ":handshake: {{ .Symbol }} {{ .Side }} Trade Execution @ {{ .Price  }}"

func (trade Trade) SlackAttachment() slack.Attachment {
	var color = "#DC143C"

	if trade.IsBuyer {
		color = "#228B22"
	}

	liquidity := trade.Liquidity()
	pretext := util.Render(slackTradeTextTemplate, trade)
	return slack.Attachment{
		// Text: text,
		Pretext: pretext,
		Color:   color,
		Fields: []slack.AttachmentField{
			{Title: "Exchange", Value: trade.Exchange, Short: true},
			{Title: "Price", Value: util.FormatFloat(trade.Price, 2), Short: true},
			{Title: "Quantity", Value: util.FormatFloat(trade.Quantity, 4), Short: true},
			{Title: "QuoteQuantity", Value: util.FormatFloat(trade.QuoteQuantity, 2)},
			{Title: "Fee", Value: util.FormatFloat(trade.Fee, 4), Short: true},
			{Title: "FeeCurrency", Value: trade.FeeCurrency, Short: true},
			{Title: "Liquidity", Value: liquidity, Short: true},
		},
		// Footer:     tradingCtx.TradeStartTime.Format(time.RFC822),
		// FooterIcon: "",
	}
}

func (trade Trade) Liquidity() (o string) {
	if trade.IsMaker {
		o += "MAKER"
	} else {
		o += "TAKER"
	}

	return o
}

func (trade Trade) Key() TradeKey {
	return TradeKey{ID: trade.ID, Side: trade.Side}
}

type TradeKey struct {
	ID   int64
	Side SideType
}

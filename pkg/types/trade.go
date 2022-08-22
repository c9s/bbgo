package types

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/util/templateutil"
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
	slice := make([]Trade, len(s.Trades))
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
	ID            uint64           `json:"id" db:"id"`
	OrderID       uint64           `json:"orderID" db:"order_id"`
	Exchange      ExchangeName     `json:"exchange" db:"exchange"`
	Price         fixedpoint.Value `json:"price" db:"price"`
	Quantity      fixedpoint.Value `json:"quantity" db:"quantity"`
	QuoteQuantity fixedpoint.Value `json:"quoteQuantity" db:"quote_quantity"`
	Symbol        string           `json:"symbol" db:"symbol"`

	Side        SideType         `json:"side" db:"side"`
	IsBuyer     bool             `json:"isBuyer" db:"is_buyer"`
	IsMaker     bool             `json:"isMaker" db:"is_maker"`
	Time        Time             `json:"tradedAt" db:"traded_at"`
	Fee         fixedpoint.Value `json:"fee" db:"fee"`
	FeeCurrency string           `json:"feeCurrency" db:"fee_currency"`

	IsMargin   bool `json:"isMargin" db:"is_margin"`
	IsFutures  bool `json:"isFutures" db:"is_futures"`
	IsIsolated bool `json:"isIsolated" db:"is_isolated"`

	// The following fields are null-able fields

	// StrategyID is the strategy that execute this trade
	StrategyID sql.NullString `json:"strategyID" db:"strategy"`

	// PnL is the profit and loss value of the executed trade
	PnL sql.NullFloat64 `json:"pnl" db:"pnl"`
}

func (trade Trade) CsvHeader() []string {
	return []string{"id", "order_id", "exchange", "symbol", "price", "quantity", "quote_quantity", "side", "is_buyer", "is_maker", "fee", "fee_currency", "time"}
}

func (trade Trade) CsvRecords() [][]string {
	return [][]string{
		{
			strconv.FormatUint(trade.ID, 10),
			strconv.FormatUint(trade.OrderID, 10),
			trade.Exchange.String(),
			trade.Symbol,
			trade.Price.String(),
			trade.Quantity.String(),
			trade.QuoteQuantity.String(),
			trade.Side.String(),
			strconv.FormatBool(trade.IsBuyer),
			strconv.FormatBool(trade.IsMaker),
			trade.Fee.String(),
			trade.FeeCurrency,
			trade.Time.Time().Format(time.RFC1123),
		},
	}
}

func (trade Trade) PositionChange() fixedpoint.Value {
	q := trade.Quantity
	switch trade.Side {
	case SideTypeSell:
		return q.Neg()

	case SideTypeBuy:
		return q

	case SideTypeSelf:
		return fixedpoint.Zero
	}
	return fixedpoint.Zero
}

/*func trimTrailingZero(a string) string {
	index := strings.Index(a, ".")
	if index == -1 {
		return a
	}

	var c byte
	var i int
	for i = len(a) - 1; i >= 0; i-- {
		c = a[i]
		if c == '0' {
			continue
		} else if c == '.' {
			return a[0:i]
		} else {
			return a[0 : i+1]
		}
	}
	return a
}

func trimTrailingZero(a float64) string {
	return trimTrailingZero(fmt.Sprintf("%f", a))
}*/

// String is for console output
func (trade Trade) String() string {
	return fmt.Sprintf("TRADE %s %s %4s %-4s @ %-6s | AMOUNT %s | FEE %s %s | OrderID %d | TID %d | %s",
		trade.Exchange.String(),
		trade.Symbol,
		trade.Side,
		trade.Quantity.String(),
		trade.Price.String(),
		trade.QuoteQuantity.String(),
		trade.Fee.String(),
		trade.FeeCurrency,
		trade.OrderID,
		trade.ID,
		trade.Time.Time().Format(time.StampMilli),
	)
}

// PlainText is used for telegram-styled messages
func (trade Trade) PlainText() string {
	return fmt.Sprintf("Trade %s %s %s %s @ %s, amount %s, fee %s %s",
		trade.Exchange.String(),
		trade.Symbol,
		trade.Side,
		trade.Quantity.String(),
		trade.Price.String(),
		trade.QuoteQuantity.String(),
		trade.Fee.String(),
		trade.FeeCurrency)
}

var slackTradeTextTemplate = ":handshake: Trade {{ .Symbol }} {{ .Side }} {{ .Quantity }} @ {{ .Price  }}"

func (trade Trade) SlackAttachment() slack.Attachment {
	var color = "#DC143C"

	if trade.IsBuyer {
		color = "#228B22"
	}

	liquidity := trade.Liquidity()
	text := templateutil.Render(slackTradeTextTemplate, trade)
	footerIcon := ExchangeFooterIcon(trade.Exchange)

	return slack.Attachment{
		Text: text,
		// Title: ...
		// Pretext: pretext,
		Color: color,
		Fields: []slack.AttachmentField{
			{Title: "Exchange", Value: trade.Exchange.String(), Short: true},
			{Title: "Price", Value: trade.Price.String(), Short: true},
			{Title: "Quantity", Value: trade.Quantity.String(), Short: true},
			{Title: "QuoteQuantity", Value: trade.QuoteQuantity.String(), Short: true},
			{Title: "Fee", Value: trade.Fee.String(), Short: true},
			{Title: "FeeCurrency", Value: trade.FeeCurrency, Short: true},
			{Title: "Liquidity", Value: liquidity, Short: true},
			{Title: "Order ID", Value: strconv.FormatUint(trade.OrderID, 10), Short: true},
		},
		FooterIcon: footerIcon,
		Footer:     strings.ToLower(trade.Exchange.String()) + templateutil.Render(" creation time {{ . }}", trade.Time.Time().Format(time.StampMilli)),
	}
}

func (trade Trade) Liquidity() (o string) {
	if trade.IsMaker {
		o = "MAKER"
	} else {
		o = "TAKER"
	}

	return o
}

func (trade Trade) Key() TradeKey {
	return TradeKey{
		Exchange: trade.Exchange,
		ID:       trade.ID,
		Side:     trade.Side,
	}
}

type TradeKey struct {
	Exchange ExchangeName
	ID       uint64
	Side     SideType
}

func (k TradeKey) String() string {
	return k.Exchange.String() + strconv.FormatUint(k.ID, 10) + k.Side.String()
}

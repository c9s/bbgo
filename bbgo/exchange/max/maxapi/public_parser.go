package max

import (
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/valyala/fastjson"
)

var ErrIncorrectBookEntryElementLength = errors.New("incorrect book entry element length")

const Buy = 1
const Sell = -1

// ParseMessage accepts the raw messages from max public websocket channels and parses them into market data
// Return types: *BookEvent, *TradeEvent, *SubscriptionEvent, *ErrorEvent
func ParseMessage(payload []byte) (interface{}, error) {
	parser := fastjson.Parser{}
	val, err := parser.ParseBytes(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse payload: "+string(payload))
	}

	if channel := string(val.GetStringBytes("c")); len(channel) > 0 {
		switch channel {
		case "book":
			return parseBookEvent(val)
		case "trade":
			return parseTradeEvent(val)
		case "user":
			return ParseUserEvent(val)
		}
	}

	eventType := string(val.GetStringBytes("e"))
	switch eventType {
	case "error":
		return parseErrorEvent(val)
	case "subscribed", "unsubscribed":
		return parseSubscriptionEvent(val)
	}

	return nil, errors.Wrapf(ErrMessageTypeNotSupported, "payload %s", payload)
}

type TradeEntry struct {
	Trend     string `json:"tr"`
	Price     string `json:"p"`
	Volume    string `json:"v"`
	Timestamp int64  `json:"T"`
}

func (e TradeEntry) Time() time.Time {
	return time.Unix(0, e.Timestamp*int64(time.Millisecond))
}

// parseTradeEntry parse the trade content payload
func parseTradeEntry(val *fastjson.Value) TradeEntry {
	return TradeEntry{
		Trend:     strings.ToLower(string(val.GetStringBytes("tr"))),
		Timestamp: val.GetInt64("T"),
		Price:     string(val.GetStringBytes("p")),
		Volume:    string(val.GetStringBytes("v")),
	}
}

type TradeEvent struct {
	Event     string       `json:"e"`
	Market    string       `json:"M"`
	Channel   string       `json:"c"`
	Trades    []TradeEntry `json:"t"`
	Timestamp int64        `json:"T"`
}

func (e *TradeEvent) Time() time.Time {
	return time.Unix(0, e.Timestamp*int64(time.Millisecond))
}

func parseTradeEvent(val *fastjson.Value) (*TradeEvent, error) {
	event := TradeEvent{
		Event:     string(val.GetStringBytes("e")),
		Market:    string(val.GetStringBytes("M")),
		Channel:   string(val.GetStringBytes("c")),
		Timestamp: val.GetInt64("T"),
	}

	for _, tradeValue := range val.GetArray("t") {
		event.Trades = append(event.Trades, parseTradeEntry(tradeValue))
	}

	return &event, nil
}

type BookEvent struct {
	Event     string `json:"e"`
	Market    string `json:"M"`
	Channel   string `json:"c"`
	Timestamp int64  `json:"t"` // Millisecond timestamp
	Bids      []BookEntry
	Asks      []BookEntry
}

func (e *BookEvent) Time() time.Time {
	return time.Unix(0, e.Timestamp*int64(time.Millisecond))
}

func parseBookEvent(val *fastjson.Value) (*BookEvent, error) {
	event := BookEvent{
		Event:     string(val.GetStringBytes("e")),
		Market:    string(val.GetStringBytes("M")),
		Channel:   string(val.GetStringBytes("c")),
		Timestamp: val.GetInt64("T"),
	}

	t := time.Unix(0, event.Timestamp*int64(time.Millisecond))

	var err error
	event.Asks, err = parseBookEntries(val.GetArray("a"), Sell, t)
	if err != nil {
		return nil, err
	}

	event.Bids, err = parseBookEntries(val.GetArray("b"), Buy, t)
	if err != nil {
		return nil, err
	}

	return &event, nil
}

type BookEntry struct {
	Side   int
	Time   time.Time
	Price  string
	Volume string
}

// parseBookEntries parses JSON struct like `[["233330", "0.33"], ....]`
func parseBookEntries(vals []*fastjson.Value, side int, t time.Time) (entries []BookEntry, err error) {
	for _, entry := range vals {
		pv, err := entry.Array()
		if err != nil {
			return nil, err
		}

		if len(pv) < 2 {
			return nil, ErrIncorrectBookEntryElementLength
		}

		entries = append(entries, BookEntry{
			Side:   side,
			Time:   t,
			Price:  pv[0].String(),
			Volume: pv[1].String(),
		})
	}

	return entries, nil
}

type ErrorEvent struct {
	Timestamp int64
	Errors    []string
	CommandID string
}

func (e ErrorEvent) Time() time.Time {
	return time.Unix(0, e.Timestamp*int64(time.Millisecond))
}

func parseErrorEvent(val *fastjson.Value) (*ErrorEvent, error) {
	event := ErrorEvent{
		Timestamp: val.GetInt64("T"),
		CommandID: string(val.GetStringBytes("i")),
	}

	for _, entry := range val.GetArray("E") {
		event.Errors = append(event.Errors, string(entry.GetStringBytes()))
	}

	return &event, nil
}

type SubscriptionEvent struct {
	Event         string         `json:"e"`
	Timestamp     int64          `json:"T"`
	CommandID     string         `json:"i"`
	Subscriptions []Subscription `json:"s"`
}

func (e SubscriptionEvent) Time() time.Time {
	return time.Unix(0, e.Timestamp*int64(time.Millisecond))
}

func parseSubscriptionEvent(val *fastjson.Value) (*SubscriptionEvent, error) {
	event := SubscriptionEvent{
		Event:     string(val.GetStringBytes("e")),
		Timestamp: val.GetInt64("T"),
		CommandID: string(val.GetStringBytes("i")),
	}

	for _, entry := range val.GetArray("s") {
		market := string(entry.GetStringBytes("market"))
		channel := string(entry.GetStringBytes("channel"))
		event.Subscriptions = append(event.Subscriptions, Subscription{
			Market:  market,
			Channel: channel,
		})
	}

	return &event, nil
}



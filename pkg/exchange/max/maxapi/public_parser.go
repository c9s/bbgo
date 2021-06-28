package max

import (
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/valyala/fastjson"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var ErrIncorrectBookEntryElementLength = errors.New("incorrect book entry element length")

const Buy = 1
const Sell = -1

var parserPool fastjson.ParserPool

// ParseMessage accepts the raw messages from max public websocket channels and parses them into market data
// Return types: *BookEvent, *PublicTradeEvent, *SubscriptionEvent, *ErrorEvent
func ParseMessage(payload []byte) (interface{}, error) {
	parser := parserPool.Get()

	val, err := parser.ParseBytes(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse payload: "+string(payload))
	}

	if channel := string(val.GetStringBytes("c")); len(channel) > 0 {
		switch channel {
		case "kline":
			return parseKLineEvent(val)
		case "book":
			return parseBookEvent(val)
		case "trade":
			return parsePublicTradeEvent(val)
		case "user":
			return ParseUserEvent(val)
		}
	}

	eventType := string(val.GetStringBytes("e"))
	switch eventType {
	case "authenticated":
		return parseAuthEvent(val)
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

type KLineEvent struct {
	Event     string `json:"e"`
	Market    string `json:"M"`
	Channel   string `json:"c"`
	KLine     KLine  `json:"k"`
	Timestamp int64  `json:"T"`
}

/*
	{
	  "c": "kline",
	  "M": "btcusdt",
	  "e": "update",
	  "T": 1602999650179,
	  "k": {
	    "ST": 1602999900000,
	    "ET": 1602999900000,
	    "M": "btcusdt",
	    "R": "5m",
	    "O": "11417.21",
	    "H": "11417.21",
	    "L": "11417.21",
	    "C": "11417.21",
	    "v": "0",
	    "ti": 0,
	    "x": false
	  }
	}
*/
type KLinePayload struct {
	StartTime   int64  `json:"ST"`
	EndTime     int64  `json:"ET"`
	Market      string `json:"M"`
	Resolution  string `json:"R"`
	Open        string `json:"O"`
	High        string `json:"H"`
	Low         string `json:"L"`
	Close       string `json:"C"`
	Volume      string `json:"v"`
	LastTradeID int    `json:"ti"`
	Closed      bool   `json:"x"`
}

func (k KLinePayload) KLine() types.KLine {
	return types.KLine{
		StartTime:      types.Time(time.Unix(0, k.StartTime*int64(time.Millisecond))),
		EndTime:        types.Time(time.Unix(0, k.EndTime*int64(time.Millisecond))),
		Symbol:         k.Market,
		Interval:       types.Interval(k.Resolution),
		Open:           fixedpoint.MustNewFromString(k.Open),
		Close:          fixedpoint.MustNewFromString(k.Close),
		High:           fixedpoint.MustNewFromString(k.High),
		Low:            fixedpoint.MustNewFromString(k.Low),
		Volume:         fixedpoint.MustNewFromString(k.Volume),
		QuoteVolume:    fixedpoint.Zero, // TODO: add this from kingfisher
		LastTradeID:    uint64(k.LastTradeID),
		NumberOfTrades: 0, // TODO: add this from kingfisher
		Closed:         k.Closed,
	}
}

type PublicTradeEvent struct {
	Event     string       `json:"e"`
	Market    string       `json:"M"`
	Channel   string       `json:"c"`
	Trades    []TradeEntry `json:"t"`
	Timestamp int64        `json:"T"`
}

func (e *PublicTradeEvent) Time() time.Time {
	return time.Unix(0, e.Timestamp*int64(time.Millisecond))
}

func parsePublicTradeEvent(val *fastjson.Value) (*PublicTradeEvent, error) {
	event := PublicTradeEvent{
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
	Bids      types.PriceVolumeSlice
	Asks      types.PriceVolumeSlice
}

func (e *BookEvent) Time() time.Time {
	return time.Unix(0, e.Timestamp*int64(time.Millisecond))
}

func (e *BookEvent) OrderBook() (snapshot types.SliceOrderBook, err error) {
	snapshot.Symbol = strings.ToUpper(e.Market)
	snapshot.Time = e.Time()
	snapshot.Bids = e.Bids
	snapshot.Asks = e.Asks
	return snapshot, nil
}

func parseKLineEvent(val *fastjson.Value) (*KLineEvent, error) {
	event := KLineEvent{
		Event:     string(val.GetStringBytes("e")),
		Market:    string(val.GetStringBytes("M")),
		Channel:   string(val.GetStringBytes("c")),
		Timestamp: val.GetInt64("T"),
	}

	k := val.Get("k")

	event.KLine = KLine{
		Symbol:    string(k.GetStringBytes("M")),
		Interval:  string(k.GetStringBytes("R")),
		StartTime: time.Unix(0, k.GetInt64("ST")*int64(time.Millisecond)),
		EndTime:   time.Unix(0, k.GetInt64("ET")*int64(time.Millisecond)),
		Open:      fixedpoint.MustNewFromBytes(k.GetStringBytes("O")),
		High:      fixedpoint.MustNewFromBytes(k.GetStringBytes("H")),
		Low:       fixedpoint.MustNewFromBytes(k.GetStringBytes("L")),
		Close:     fixedpoint.MustNewFromBytes(k.GetStringBytes("C")),
		Volume:    fixedpoint.MustNewFromBytes(k.GetStringBytes("v")),
		Closed:    k.GetBool("x"),
	}

	return &event, nil
}

func parseBookEvent(val *fastjson.Value) (event *BookEvent, err error) {
	event = &BookEvent{
		Event:     string(val.GetStringBytes("e")),
		Market:    string(val.GetStringBytes("M")),
		Channel:   string(val.GetStringBytes("c")),
		Timestamp: val.GetInt64("T"),
	}

	// t := time.Unix(0, event.Timestamp*int64(time.Millisecond))
	event.Asks, err = parseBookEntries2(val.GetArray("a"))
	if err != nil {
		return event, err
	}

	event.Bids, err = parseBookEntries2(val.GetArray("b"))
	return event, err
}

// parseBookEntries2 parses JSON struct like `[["233330", "0.33"], ....]`
func parseBookEntries2(vals []*fastjson.Value) (entries types.PriceVolumeSlice, err error) {
	entries = make(types.PriceVolumeSlice, 0, 50)

	var arr []*fastjson.Value
	for _, entry := range vals {
		arr, err = entry.Array()
		if err != nil {
			return entries, err
		}

		if len(arr) < 2 {
			return entries, ErrIncorrectBookEntryElementLength
		}

		var pv types.PriceVolume
		pv.Price, err = fixedpoint.NewFromString(string(arr[0].GetStringBytes()))
		if err != nil {
			return entries, err
		}

		pv.Volume, err = fixedpoint.NewFromString(string(arr[1].GetStringBytes()))
		if err != nil {
			return entries, err
		}

		entries = append(entries, pv)
	}

	return entries, err
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

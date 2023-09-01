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

// ParseMessage accepts the raw messages from max public websocket channels and parses them into market data
// Return types: *BookEvent, *PublicTradeEvent, *SubscriptionEvent, *ErrorEvent
func ParseMessage(payload []byte) (interface{}, error) {
	parser := fastjson.Parser{}
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
	Bids      []BookEntry
	Asks      []BookEntry
}

func (e *BookEvent) Time() time.Time {
	return time.Unix(0, e.Timestamp*int64(time.Millisecond))
}

func (e *BookEvent) OrderBook() (snapshot types.SliceOrderBook, err error) {
	snapshot.Symbol = strings.ToUpper(e.Market)
	snapshot.Time = e.Time()

	for _, bid := range e.Bids {
		pv, err := bid.PriceVolumePair()
		if err != nil {
			return snapshot, err
		}

		snapshot.Bids = append(snapshot.Bids, pv)
	}

	for _, ask := range e.Asks {
		pv, err := ask.PriceVolumePair()
		if err != nil {
			return snapshot, err
		}

		snapshot.Asks = append(snapshot.Asks, pv)
	}

	return snapshot, nil
}

func parseKLineEvent(val *fastjson.Value) (*KLineEvent, error) {
	event := KLineEvent{
		Event:     string(val.GetStringBytes("e")),
		Market:    string(val.GetStringBytes("M")),
		Channel:   string(val.GetStringBytes("c")),
		Timestamp: val.GetInt64("T"),
	}

	event.KLine = KLine{
		Symbol:    string(val.GetStringBytes("k", "M")),
		Interval:  string(val.GetStringBytes("k", "R")),
		StartTime: time.Unix(0, val.GetInt64("k", "ST")*int64(time.Millisecond)),
		EndTime:   time.Unix(0, val.GetInt64("k", "ET")*int64(time.Millisecond)),
		Open:      fixedpoint.MustNewFromBytes(val.GetStringBytes("k", "O")),
		High:      fixedpoint.MustNewFromBytes(val.GetStringBytes("k", "H")),
		Low:       fixedpoint.MustNewFromBytes(val.GetStringBytes("k", "L")),
		Close:     fixedpoint.MustNewFromBytes(val.GetStringBytes("k", "C")),
		Volume:    fixedpoint.MustNewFromBytes(val.GetStringBytes("k", "v")),
		Closed:    val.GetBool("k", "x"),
	}

	return &event, nil
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

func (e *BookEntry) PriceVolumePair() (pv types.PriceVolume, err error) {
	pv.Price, err = fixedpoint.NewFromString(e.Price)
	if err != nil {
		return pv, err
	}

	pv.Volume, err = fixedpoint.NewFromString(e.Volume)
	if err != nil {
		return pv, err
	}

	return pv, err
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
			Price:  string(pv[0].GetStringBytes()),
			Volume: string(pv[1].GetStringBytes()),
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

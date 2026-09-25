// Package marketdata defines a provider-agnostic, time-ordered market data
// ingestion layer.
//
// The layer has three pieces:
//
//   - Event, a tagged union of every market data observation we can replay,
//     carrying an OrderKey that totally orders it against every other event.
//   - Source and Cursor, a pull-based iterator contract that a provider
//     (CSV archives, a REST vendor, a gRPC stream, a local recording)
//     implements.
//   - Merge, a k-way heap merge that interleaves several cursors into a single
//     non-decreasing stream.
//
// The core package imports only pkg/types, pkg/fixedpoint and the standard
// library. Providers live under pkg/marketdata/sources and import this package,
// never the other way around.
package marketdata

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// EventType tags the populated payload field of an Event.
type EventType uint8

const (
	EventTypeUnknown EventType = iota

	// EventTypeKLine is a closed candlestick. Its OrderKey.TimeNs is the close
	// time, not the open time.
	EventTypeKLine

	// EventTypeTrade is a public trade or an aggregated trade. Aggregated
	// trades additionally carry FlagAggregated.
	EventTypeTrade

	// EventTypeBookSnapshot is a complete order book state.
	EventTypeBookSnapshot

	// EventTypeBookUpdate is an incremental order book diff. A level with zero
	// volume means "remove this level", matching both Binance and AmberData.
	EventTypeBookUpdate

	// EventTypeBookTicker is the L1 best bid/ask.
	EventTypeBookTicker

	EventTypeMarkPrice
	EventTypeLiquidation

	// EventTypeMetrics is a periodic scalar series: open interest, long/short
	// ratios, funding.
	EventTypeMetrics

	// EventTypeDepthBand is one Binance futures bookDepth row: aggregate
	// notional within a percentage band of mid. It is NOT an order book and
	// must never be applied to a BookState.
	EventTypeDepthBand
)

var eventTypeNames = map[EventType]string{
	EventTypeUnknown:      "unknown",
	EventTypeKLine:        "kline",
	EventTypeTrade:        "trade",
	EventTypeBookSnapshot: "bookSnapshot",
	EventTypeBookUpdate:   "bookUpdate",
	EventTypeBookTicker:   "bookTicker",
	EventTypeMarkPrice:    "markPrice",
	EventTypeLiquidation:  "liquidation",
	EventTypeMetrics:      "metrics",
	EventTypeDepthBand:    "depthBand",
}

func (t EventType) String() string {
	if s, ok := eventTypeNames[t]; ok {
		return s
	}
	return "unknown"
}

// EventFlag carries out-of-band facts a consumer needs but that do not belong
// in the payload.
type EventFlag uint16

const (
	// FlagSynthetic marks an event bbgo derived rather than one the venue
	// published, for example a one-level book synthesized from a bookTicker row.
	FlagSynthetic EventFlag = 1 << iota

	// FlagNeedsSnapshot marks a book update emitted without a preceding
	// snapshot for that symbol. The consumer must not apply it to an
	// uninitialized book.
	FlagNeedsSnapshot

	// FlagAggregated marks a trade that aggregates several matches, such as a
	// Binance aggTrade.
	FlagAggregated

	// FlagPartialDepth marks a book payload truncated to top-N levels.
	FlagPartialDepth
)

func (f EventFlag) Has(x EventFlag) bool { return f&x != 0 }

// Event is a single timestamped market data observation. Exactly one payload
// pointer is non-nil, selected by Type. The zero Event is invalid.
//
// An Event returned by a Cursor is valid only until the next call to
// Cursor.Next; a consumer that retains it must Clone it first.
type Event struct {
	// Key is the total ordering key. See OrderKey.
	Key OrderKey

	Type     EventType
	Flags    EventFlag
	Exchange types.ExchangeName
	Symbol   string

	// Source is the Source.Name() that produced this event. It is set by the
	// merge, not by decoders.
	Source string

	KLine       *types.KLine
	Trade       *types.Trade
	Book        *types.SliceOrderBook // EventTypeBookSnapshot and EventTypeBookUpdate
	BookTicker  *BookTicker
	MarkPrice   *MarkPrice
	Liquidation *types.LiquidationInfo
	Metrics     *Metrics
	DepthBand   *DepthBand

	// Raw carries provider-specific data that has no home above. Consumers
	// must type-assert it; the merge never looks at it.
	Raw any
}

// Time returns the event time as a UTC time.Time.
func (e *Event) Time() time.Time { return time.Unix(0, e.Key.TimeNs).UTC() }

// Clone deep-copies the event and its selected payload, so the result stays
// valid after the producing cursor advances.
func (e *Event) Clone() *Event {
	c := *e

	switch {
	case e.KLine != nil:
		k := *e.KLine
		c.KLine = &k
	case e.Trade != nil:
		t := *e.Trade
		c.Trade = &t
	case e.Book != nil:
		b := *e.Book
		b.Asks = append(types.PriceVolumeSlice(nil), e.Book.Asks...)
		b.Bids = append(types.PriceVolumeSlice(nil), e.Book.Bids...)
		c.Book = &b
	case e.BookTicker != nil:
		bt := *e.BookTicker
		c.BookTicker = &bt
	case e.MarkPrice != nil:
		mp := *e.MarkPrice
		c.MarkPrice = &mp
	case e.Liquidation != nil:
		l := *e.Liquidation
		c.Liquidation = &l
	case e.Metrics != nil:
		m := *e.Metrics
		m.Values = make(map[string]fixedpoint.Value, len(e.Metrics.Values))
		for k, v := range e.Metrics.Values {
			m.Values[k] = v
		}
		c.Metrics = &m
	case e.DepthBand != nil:
		d := *e.DepthBand
		c.DepthBand = &d
	}

	return &c
}

// BookTicker is types.BookTicker plus the timing information a time-ordered
// replay needs.
//
// types.BookTicker itself has no timestamp — the field is commented out in
// pkg/types/bookticker.go. Adding one there is the cleaner long-term fix but
// touches every exchange adapter that constructs one, so it is deliberately out
// of scope here.
type BookTicker struct {
	types.BookTicker

	UpdateID int64

	// TransactionTime is when the book changed in the venue matching engine.
	// OrderKey.TimeNs is derived from this one.
	TransactionTime types.Time

	// EventTime is when the venue published the update. The difference between
	// the two is publication latency, which a realistic backtest will
	// eventually want to model.
	EventTime types.Time
}

// MarkPrice is a futures mark price observation.
type MarkPrice struct {
	Symbol               string
	MarkPrice            fixedpoint.Value
	IndexPrice           fixedpoint.Value
	EstimatedSettlePrice fixedpoint.Value
	LastFundingRate      fixedpoint.Value
	NextFundingTime      types.Time
	Time                 types.Time
}

// Metrics is the generic slot for periodic scalar series such as open interest
// and long/short ratios. It is keyed by name so a new series needs no type
// change.
type Metrics struct {
	Symbol string
	Time   types.Time
	Values map[string]fixedpoint.Value
}

// Metric key constants for the Binance futures "metrics" dataset.
const (
	MetricSumOpenInterest              = "sumOpenInterest"
	MetricSumOpenInterestValue         = "sumOpenInterestValue"
	MetricCountTopTraderLongShortRatio = "countTopTraderLongShortRatio"
	MetricSumTopTraderLongShortRatio   = "sumTopTraderLongShortRatio"
	MetricCountLongShortRatio          = "countLongShortRatio"
	MetricSumTakerLongShortVolRatio    = "sumTakerLongShortVolRatio"
)

// DepthBand is one row of the Binance futures bookDepth dataset: the aggregate
// notional resting within Percentage percent of mid.
//
// It is not an order book: it has no price levels. It exists so the dataset can
// be replayed alongside everything else without being mistaken for L2.
type DepthBand struct {
	Symbol     string
	Time       types.Time
	Percentage fixedpoint.Value // -5, -4, -3, -2, -1, 1, 2, 3, 4, 5
	Depth      fixedpoint.Value
	Notional   fixedpoint.Value
}

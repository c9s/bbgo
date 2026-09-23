// Package amberdataapi is a REST client for AmberData's market data API.
//
// It was written from the OpenAPI specifications AmberData publishes openly at
// docs.amberdata.io — every documentation page has a .md twin with the complete
// spec inline — because the API itself has no free tier, no sandbox and no demo
// key. Requests without a key return an AWS API Gateway 403, so the request
// builders and response types here are derived from the specs and the response
// examples in them, and exercised against those examples as fixtures.
//
// What that leaves unverified is listed in the package comment of the parent
// amberdata package. Anything uncertain sits behind an interface there so it can
// be corrected in one place.
package amberdataapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// Response is the envelope every endpoint wraps its payload in.
type Response[T any] struct {
	Status      int    `json:"status"`
	Title       string `json:"title"`
	Description string `json:"description"`

	Payload struct {
		Metadata Metadata `json:"metadata"`
		Data     []T      `json:"data"`
	} `json:"payload"`
}

// Metadata carries the pagination cursor and the range the server actually
// served, which is worth logging: a request can legally return less than it asked
// for.
type Metadata struct {
	// Next is a complete absolute URL with an opaque compressed cursor. It must
	// be re-issued verbatim; it cannot be reconstructed. Empty when exhausted.
	Next string `json:"next"`

	APIVersion string `json:"api-version"`

	RequestedStartDate Timestamp `json:"requestedStartDate"`
	RequestedEndDate   Timestamp `json:"requestedEndDate"`
	ReturnedStartDate  Timestamp `json:"returnedStartDate"`
	ReturnedEndDate    Timestamp `json:"returnedEndDate"`
}

// Timestamp decodes a timestamp whose encoding depends on the timeFormat
// parameter.
//
// The client always sends timeFormat=milliseconds, so this should always be an
// integer — but the API's default is `hr`, which emits "2024-06-04 16:23:12 414":
// a space-separated millisecond field that is not RFC3339 and that several of the
// published response examples use. Accepting both means a fixture transcribed
// from the docs decodes, and a request that somehow loses the parameter degrades
// instead of failing.
type Timestamp struct {
	time.Time
}

// hrDateTimeLayout is the date and time part of AmberData's "human readable"
// format. The format's milliseconds are separated by a space —
// "2024-06-04 16:23:12 414" — which a Go layout cannot express, because a
// fractional second must directly follow the seconds after a "." or ",". So the
// millisecond field is split off and added separately; see parseHumanReadable.
const hrDateTimeLayout = "2006-01-02 15:04:05"

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == `""` || s == "" {
		return nil
	}

	if s[0] == '"' {
		var raw string
		if err := json.Unmarshal(b, &raw); err != nil {
			return err
		}
		return t.parseString(raw)
	}

	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		// A float is possible if the server ever emits fractional milliseconds.
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return fmt.Errorf("amberdata: cannot parse timestamp %s: %w", s, err)
		}
		t.Time = time.UnixMilli(int64(f)).UTC()
		return nil
	}

	t.Time = time.UnixMilli(ms).UTC()
	return nil
}

func (t *Timestamp) parseString(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	if ms, err := strconv.ParseInt(raw, 10, 64); err == nil {
		t.Time = time.UnixMilli(ms).UTC()
		return nil
	}

	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		hrDateTimeLayout,
		"2006-01-02T15:04:05",
	} {
		if parsed, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			t.Time = parsed.UTC()
			return nil
		}
	}

	if parsed, ok := parseHumanReadable(raw); ok {
		t.Time = parsed
		return nil
	}

	return fmt.Errorf("amberdata: cannot parse timestamp %q", raw)
}

// parseHumanReadable parses AmberData's default timeFormat, "hr", which looks
// like "2024-06-04 16:23:12 414": a date, a time, then milliseconds after a
// space.
//
// This cannot be done with a layout string, because Go only recognizes a
// fractional second when it directly follows the seconds after a "." or ",". The
// client always requests milliseconds instead, so this path exists for the
// response examples in the published specifications and for the case where the
// parameter is somehow lost.
func parseHumanReadable(raw string) (time.Time, bool) {
	idx := strings.LastIndexByte(raw, ' ')
	if idx < 0 {
		return time.Time{}, false
	}

	head, tail := raw[:idx], raw[idx+1:]

	base, err := time.ParseInLocation(hrDateTimeLayout, head, time.UTC)
	if err != nil {
		return time.Time{}, false
	}

	millis, err := strconv.Atoi(tail)
	if err != nil || millis < 0 || millis > 999 {
		return time.Time{}, false
	}

	return base.Add(time.Duration(millis) * time.Millisecond), true
}

// Number decodes a numeric field that the API types inconsistently.
//
// The specifications declare prices as `number` but sequences as `number` in one
// endpoint and emit them as quoted strings in another, and any of them can be
// null. Rather than guess per field, every numeric value goes through this.
type Number struct {
	fixedpoint.Value
	Valid bool
}

func (n *Number) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == "" {
		return nil
	}

	if s[0] == '"' {
		var raw string
		if err := json.Unmarshal(b, &raw); err != nil {
			return err
		}
		if raw == "" {
			return nil
		}
		v, err := fixedpoint.NewFromString(raw)
		if err != nil {
			return fmt.Errorf("amberdata: cannot parse number %q: %w", raw, err)
		}
		n.Value, n.Valid = v, true
		return nil
	}

	v, err := fixedpoint.NewFromString(s)
	if err != nil {
		return fmt.Errorf("amberdata: cannot parse number %s: %w", s, err)
	}
	n.Value, n.Valid = v, true
	return nil
}

// Uint64 decodes an integer that may arrive quoted or null, which is how
// sequences and trade ids behave across these endpoints.
type Uint64 struct {
	Value uint64
	Valid bool
}

func (u *Uint64) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "null" || s == "" {
		return nil
	}

	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		// Some ids are not numeric at all on some venues. Losing the value is
		// better than failing the record; it is only used as a tiebreaker.
		return nil
	}

	u.Value, u.Valid = v, true
	return nil
}

// Trade is one record of /futures/trades and /spot/trades.
type Trade struct {
	Instrument string `json:"instrument"`
	Exchange   string `json:"exchange"`

	ExchangeTimestamp            Timestamp `json:"exchangeTimestamp"`
	ExchangeTimestampNanoseconds int64     `json:"exchangeTimestampNanoseconds"`

	// IsBuySide reports the aggressor side, so unlike Binance's is_buyer_maker
	// it needs no inversion.
	IsBuySide bool `json:"isBuySide"`

	Price       Number `json:"price"`
	Volume      Number `json:"volume"`
	QuoteVolume Number `json:"quoteVolume"`

	TradeID  string `json:"tradeId"`
	Sequence Uint64 `json:"sequence"`
}

// PriceLevel is one order book level. In an order-book-events record a level
// with Volume zero means the level was removed, which matches
// types.SliceOrderBook.Update exactly.
type PriceLevel struct {
	Price     Number `json:"price"`
	Volume    Number `json:"volume"`
	NumOrders *int   `json:"numOrders"`
}

// Book is one record of order-book-snapshots and order-book-events. The two
// endpoints return the same shape; there is no flag distinguishing them, so
// snapshot versus update is decided by which endpoint was called.
type Book struct {
	Exchange   string `json:"exchange"`
	Instrument string `json:"instrument"`

	Timestamp                    Timestamp `json:"timestamp"`
	ExchangeTimestamp            Timestamp `json:"exchangeTimestamp"`
	ExchangeTimestampNanoseconds int64     `json:"exchangeTimestampNanoseconds"`

	Sequence       Uint64 `json:"sequence"`
	CurrentFunding Number `json:"currentFunding"`

	Ask []PriceLevel `json:"ask"`
	Bid []PriceLevel `json:"bid"`
}

// OHLCV is one record of the ohlcv endpoints.
type OHLCV struct {
	Instrument string `json:"instrument"`
	Exchange   string `json:"exchange"`

	ExchangeTimestamp Timestamp `json:"exchangeTimestamp"`

	Open   Number `json:"open"`
	High   Number `json:"high"`
	Low    Number `json:"low"`
	Close  Number `json:"close"`
	Volume Number `json:"volume"`
}

// InstrumentCoverage is one record of the trades/information endpoints, which is
// how a backfill finds out what range actually exists.
type InstrumentCoverage struct {
	Exchange   string    `json:"exchange"`
	Instrument string    `json:"instrument"`
	StartDate  Timestamp `json:"startDate"`
	EndDate    Timestamp `json:"endDate"`
}

// InstrumentReference is one record of the exchanges/reference endpoints. It
// carries the contract sizing needed to interpret volumes.
type InstrumentReference struct {
	Exchange   string `json:"exchange"`
	Instrument string `json:"instrument"`

	BaseSymbol  string `json:"baseSymbol"`
	QuoteSymbol string `json:"quoteSymbol"`

	ContractPeriod     string `json:"contractPeriod"`
	ContractSettleType string `json:"contractSettleType"`
	ContractSize       Number `json:"contractSize"`

	PrecisionPrice  Number `json:"precisionPrice"`
	PrecisionVolume Number `json:"precisionVolume"`

	// UnderlyingToPositionMultiplier converts a contract count into base asset
	// units. BitMEX reports volume in contracts rather than base asset, and
	// whether Binance futures does the same is unverified, so this is the field
	// that settles it once a key exists.
	UnderlyingToPositionMultiplier Number `json:"underlyingToPositionMultiplier"`
}

// EventTimeNano returns the trade's event time in nanoseconds, folding in the
// sub-millisecond part the API reports separately.
func (t Trade) EventTimeNano() int64 {
	return t.ExchangeTimestamp.UnixNano() + t.ExchangeTimestampNanoseconds
}

// EventTimeNano returns the book record's event time in nanoseconds. The
// exchange timestamp is preferred over the ingestion timestamp: only the former
// is meaningful under simulated time.
func (b Book) EventTimeNano() int64 {
	if !b.ExchangeTimestamp.IsZero() {
		return b.ExchangeTimestamp.UnixNano() + b.ExchangeTimestampNanoseconds
	}
	return b.Timestamp.UnixNano()
}

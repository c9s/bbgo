// Package replay records a live market data stream to disk and plays it back as
// a marketdata.Source.
//
// It exists because the public archives cannot supply L2. data.binance.vision
// publishes no depth diffs, and the vendor APIs that do need a paid key, so the
// only way to get real order book snapshots and updates into a backtest without
// buying data is to capture them from the venue's websocket feed and replay
// them. That also makes the L2 half of the market data layer testable end to
// end: record a few minutes, replay it, apply it to a book, and assert the
// sequence never breaks.
package replay

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// FormatVersion is the recording format version. A reader refuses a file it
// does not understand rather than guessing.
const FormatVersion = 1

// Header is the first line of every recording file.
type Header struct {
	Version   int                `json:"v"`
	Exchange  types.ExchangeName `json:"exchange"`
	Symbols   []string           `json:"symbols,omitempty"`
	Channels  []types.Channel    `json:"channels,omitempty"`
	StartedAt int64              `json:"startedAt"` // nanoseconds
	Note      string             `json:"note,omitempty"`
}

// Record is one recorded event.
//
// The field names are short because this is written at websocket rate, and the
// schema version lives in the header rather than in every line. JSON Lines was
// chosen over a binary format deliberately: a recording can be inspected with
// `zcat | head`, needs no new dependency, and streams without a full parse. The
// size cost is largely recovered by gzip.
type Record struct {
	// T is the event time in nanoseconds since the Unix epoch.
	T int64 `json:"t"`

	// Ty is the event type. Book snapshot versus update is encoded here, which
	// is the distinction a replay has to preserve.
	Ty marketdata.EventType `json:"ty"`

	// Ex and S override the header's exchange and symbol, so one file can hold
	// several symbols.
	Ex types.ExchangeName `json:"ex,omitempty"`
	S  string             `json:"s,omitempty"`

	// Q is the venue sequence: SliceOrderBook.LastUpdateId, or a trade id.
	Q uint64 `json:"q,omitempty"`

	// U and Pu are the venue's first and previous update ids, recorded when
	// available so a replay can prove diff contiguity rather than guess at it.
	//
	// They are usually absent in a recording made today: Binance sends them,
	// and pkg/depth carries them on depth.Update, but the stream emits only the
	// inner SliceOrderBook, so a types.Stream consumer never sees them. A
	// replayed book therefore validates as monotonic-but-unverified. Exposing
	// them through the stream is the follow-up that closes this.
	U  int64 `json:"U,omitempty"`
	Pu int64 `json:"pu,omitempty"`

	// F carries the event flags.
	F marketdata.EventFlag `json:"f,omitempty"`

	// P is the payload, shaped by Ty.
	P json.RawMessage `json:"p"`
}

// bookPayload is the on-disk shape of a book snapshot or update. Price levels
// are written as two-element arrays, which is both compact and how every venue
// sends them.
type bookPayload struct {
	Bids [][2]string `json:"b,omitempty"`
	Asks [][2]string `json:"a,omitempty"`
}

// tradePayload is the on-disk shape of a public trade.
type tradePayload struct {
	Price    string `json:"p"`
	Quantity string `json:"q"`
	Buy      bool   `json:"buy"`
	ID       uint64 `json:"id,omitempty"`
}

// klinePayload is the on-disk shape of a closed kline.
type klinePayload struct {
	Interval    types.Interval `json:"i"`
	StartTime   int64          `json:"st"`
	Open        string         `json:"o"`
	High        string         `json:"h"`
	Low         string         `json:"l"`
	Close       string         `json:"c"`
	Volume      string         `json:"v"`
	QuoteVolume string         `json:"qv,omitempty"`
}

// bookTickerPayload is the on-disk shape of an L1 update.
type bookTickerPayload struct {
	BidPrice string `json:"bp"`
	BidSize  string `json:"bs"`
	AskPrice string `json:"ap"`
	AskSize  string `json:"as"`
	EventT   int64  `json:"et,omitempty"`
}

func levelsToPairs(levels types.PriceVolumeSlice) [][2]string {
	if len(levels) == 0 {
		return nil
	}

	out := make([][2]string, len(levels))
	for i, pv := range levels {
		out[i] = [2]string{pv.Price.String(), pv.Volume.String()}
	}
	return out
}

func pairsToLevels(pairs [][2]string) (types.PriceVolumeSlice, error) {
	if len(pairs) == 0 {
		return nil, nil
	}

	out := make(types.PriceVolumeSlice, len(pairs))
	for i, pair := range pairs {
		price, err := fixedpoint.NewFromString(pair[0])
		if err != nil {
			return nil, fmt.Errorf("replay: bad price %q: %w", pair[0], err)
		}
		volume, err := fixedpoint.NewFromString(pair[1])
		if err != nil {
			return nil, fmt.Errorf("replay: bad volume %q: %w", pair[1], err)
		}
		out[i] = types.PriceVolume{Price: price, Volume: volume}
	}
	return out, nil
}

func nanoTime(ns int64) time.Time { return time.Unix(0, ns).UTC() }

func recordError(r Record, err error) error {
	return fmt.Errorf("replay: %s record at %s: %w", r.Ty, nanoTime(r.T).Format(time.RFC3339Nano), err)
}

// nowNs is the arrival-time fallback for events whose venue timestamp is
// missing. It is a variable so tests can make recordings deterministic.
var nowNs = func() int64 { return time.Now().UnixNano() }

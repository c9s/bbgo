package marketdata

import (
	"cmp"

	"github.com/c9s/bbgo/pkg/types"
)

// OrderKey is the total order over all events from all sources. It is compared
// lexicographically: TimeNs, then Rank, then Seq, then SourceIndex.
//
// It is a plain comparable struct so the merge heap can order events without
// ever dereferencing a payload pointer.
type OrderKey struct {
	// TimeNs is the venue event time in nanoseconds since the Unix epoch, UTC.
	//
	// Nanoseconds rather than milliseconds because AmberData publishes
	// exchangeTimestampNanoseconds, Binance spot archives switched to
	// microseconds, and sub-millisecond ordering is the point of tick data.
	TimeNs int64

	// Rank orders event classes that share a TimeNs. Lower is delivered first.
	// See the Rank constants and KLineRank.
	Rank int64

	// Seq is the venue-provided monotonic sequence within (TimeNs, Rank):
	// an aggregate trade id, a SliceOrderBook.LastUpdateId, an AmberData
	// sequence. Zero when the venue provides none.
	Seq uint64

	// SourceIndex is assigned by the merge from the source's position in its
	// input slice, and makes the order total and stable when everything else
	// ties. A Source implementation must leave it zero.
	SourceIndex int32
}

// Compare orders two keys. It returns a negative number, zero, or a positive
// number as k sorts before, equal to, or after o.
func (k OrderKey) Compare(o OrderKey) int {
	if k.TimeNs != o.TimeNs {
		return cmp.Compare(k.TimeNs, o.TimeNs)
	}
	if k.Rank != o.Rank {
		return cmp.Compare(k.Rank, o.Rank)
	}
	if k.Seq != o.Seq {
		return cmp.Compare(k.Seq, o.Seq)
	}
	return cmp.Compare(k.SourceIndex, o.SourceIndex)
}

// Before reports whether k sorts strictly before o.
func (k OrderKey) Before(o OrderKey) bool { return k.Compare(o) < 0 }

// Rank constants order event classes that share an event time.
//
// The ordering encodes causality: at an identical timestamp a trade is the
// cause and the resulting book delta is the effect, so trades come first. A
// snapshot is the base state that same-timestamp updates apply on top of, so it
// precedes updates. Klines come last, because a kline closing at T summarizes
// everything that happened up to and including T.
const (
	RankTrade        int64 = 100
	RankBookSnapshot int64 = 200
	RankBookUpdate   int64 = 300
	RankBookTicker   int64 = 400
	RankMarkPrice    int64 = 500
	RankLiquidation  int64 = 600
	RankMetrics      int64 = 700
	RankDepthBand    int64 = 800

	// rankKLineBase is the floor for kline ranks; see KLineRank.
	rankKLineBase int64 = 1_000_000
)

// KLineRank returns the OrderKey.Rank for a closed kline of the given interval.
//
// INVARIANT: at an identical close time a shorter interval MUST be delivered
// before a longer one. The matching engine consumes the required interval (1m,
// or 1s) to advance simulated time and generate fills; if the 1h kline closing
// at the same instant arrived first, strategies would observe the 1h close
// price before the engine had processed the 1m bar that produced it.
//
// This reproduces the deliberate "ORDER BY end_time ASC, start_time DESC" in
// BacktestService.QueryKLinesCh (pkg/service/backtest_db.go): same end_time,
// later start_time first, which is the same thing as shorter interval first.
//
// An empty interval ranks at the base, before every real interval.
// types.Interval.Milliseconds panics on an unparseable interval, so the empty
// case is handled here rather than propagating.
func KLineRank(interval types.Interval) int64 {
	if len(interval) == 0 {
		return rankKLineBase
	}
	return rankKLineBase + int64(interval.Milliseconds())
}

// RankOf returns the Rank for an event type that does not depend on further
// parameters. Klines are excluded: use KLineRank, which needs the interval.
func RankOf(t EventType) int64 {
	switch t {
	case EventTypeTrade:
		return RankTrade
	case EventTypeBookSnapshot:
		return RankBookSnapshot
	case EventTypeBookUpdate:
		return RankBookUpdate
	case EventTypeBookTicker:
		return RankBookTicker
	case EventTypeMarkPrice:
		return RankMarkPrice
	case EventTypeLiquidation:
		return RankLiquidation
	case EventTypeMetrics:
		return RankMetrics
	case EventTypeDepthBand:
		return RankDepthBand
	case EventTypeKLine:
		return rankKLineBase
	default:
		return 0
	}
}

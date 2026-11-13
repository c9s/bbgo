package marketdata

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestOrderKey_Compare(t *testing.T) {
	tests := []struct {
		name string
		a, b OrderKey
		want int // -1, 0, 1
	}{
		{
			name: "time dominates rank",
			a:    OrderKey{TimeNs: 1000, Rank: rankKLineBase},
			b:    OrderKey{TimeNs: 2000, Rank: RankTrade},
			want: -1,
		},
		{
			name: "same time, rank decides",
			a:    OrderKey{TimeNs: 1000, Rank: RankTrade},
			b:    OrderKey{TimeNs: 1000, Rank: RankBookUpdate},
			want: -1,
		},
		{
			name: "same time and rank, seq decides",
			a:    OrderKey{TimeNs: 1000, Rank: RankTrade, Seq: 5},
			b:    OrderKey{TimeNs: 1000, Rank: RankTrade, Seq: 6},
			want: -1,
		},
		{
			name: "everything else equal, source index decides",
			a:    OrderKey{TimeNs: 1000, Rank: RankTrade, Seq: 5, SourceIndex: 0},
			b:    OrderKey{TimeNs: 1000, Rank: RankTrade, Seq: 5, SourceIndex: 1},
			want: -1,
		},
		{
			name: "fully equal",
			a:    OrderKey{TimeNs: 1000, Rank: RankTrade, Seq: 5, SourceIndex: 2},
			b:    OrderKey{TimeNs: 1000, Rank: RankTrade, Seq: 5, SourceIndex: 2},
			want: 0,
		},
		{
			name: "nanosecond precision below the millisecond",
			a:    OrderKey{TimeNs: 1_000_000_001},
			b:    OrderKey{TimeNs: 1_000_000_002},
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Compare(tt.b)
			switch tt.want {
			case -1:
				assert.Negative(t, got)
				assert.Positive(t, tt.b.Compare(tt.a), "compare must be antisymmetric")
				assert.True(t, tt.a.Before(tt.b))
			case 0:
				assert.Zero(t, got)
				assert.False(t, tt.a.Before(tt.b))
			case 1:
				assert.Positive(t, got)
			}
		})
	}
}

// TestOrderKey_KLineIntervalOrdering pins the invariant that the whole ranking
// scheme exists to preserve: at an identical close time, a shorter interval is
// delivered before a longer one.
//
// This reproduces "ORDER BY end_time ASC, start_time DESC" in
// BacktestService.QueryKLinesCh (pkg/service/backtest_db.go). The matching
// engine advances simulated time on the required 1m/1s kline; if the 1h kline
// closing at the same instant arrived first, a strategy would see the 1h close
// before the engine had processed the 1m bar that produced it.
func TestOrderKey_KLineIntervalOrdering(t *testing.T) {
	const closeTime = int64(1_700_000_000_000_000_000)

	intervals := []types.Interval{
		types.Interval1s,
		types.Interval1m,
		types.Interval5m,
		types.Interval1h,
		types.Interval1d,
		types.Interval1mo,
	}

	for i := 0; i+1 < len(intervals); i++ {
		shorter := OrderKey{TimeNs: closeTime, Rank: KLineRank(intervals[i])}
		longer := OrderKey{TimeNs: closeTime, Rank: KLineRank(intervals[i+1])}

		assert.True(t, shorter.Before(longer),
			"%s must be delivered before %s at the same close time",
			intervals[i], intervals[i+1])
	}
}

// TestKLineRank_OverflowsInt32 guards the choice of int64 for Rank: a one-month
// interval is ~2.59e9 milliseconds, which does not fit in an int32.
func TestKLineRank_OverflowsInt32(t *testing.T) {
	ms := types.Interval1mo.Milliseconds()
	assert.Greater(t, ms, 1<<31-1, "1mo in ms must exceed int32, or Rank could be int32")
	assert.Equal(t, rankKLineBase+int64(ms), KLineRank(types.Interval1mo))
}

// TestKLineRank_EmptyInterval documents that an empty interval does not panic.
// types.Interval.Milliseconds panics on an unparseable interval, so KLineRank
// handles the empty case itself.
func TestKLineRank_EmptyInterval(t *testing.T) {
	assert.NotPanics(t, func() {
		assert.Equal(t, rankKLineBase, KLineRank(""))
	})
}

// TestRankOrdering_Causality pins the class ordering: a trade is the cause and
// the book delta the effect, a snapshot is the base state updates apply on top
// of, and klines come last because they summarize the interval that just ended.
func TestRankOrdering_Causality(t *testing.T) {
	ranks := []int64{
		RankTrade,
		RankBookSnapshot,
		RankBookUpdate,
		RankBookTicker,
		RankMarkPrice,
		RankLiquidation,
		RankMetrics,
		RankDepthBand,
		KLineRank(types.Interval1s),
	}

	for i := 0; i+1 < len(ranks); i++ {
		assert.Less(t, ranks[i], ranks[i+1], "rank %d must sort before rank %d", i, i+1)
	}
}

func TestRankOf(t *testing.T) {
	assert.Equal(t, RankTrade, RankOf(EventTypeTrade))
	assert.Equal(t, RankBookSnapshot, RankOf(EventTypeBookSnapshot))
	assert.Equal(t, RankBookUpdate, RankOf(EventTypeBookUpdate))
	assert.Equal(t, RankDepthBand, RankOf(EventTypeDepthBand))
	assert.Equal(t, int64(0), RankOf(EventTypeUnknown))
}

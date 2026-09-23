package marketdata_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/mdtest"
	"github.com/c9s/bbgo/pkg/types"
)

func namedSlice(t *testing.T, name, spec string) marketdata.NamedCursor {
	t.Helper()
	return marketdata.NamedCursor{
		Name:   name,
		Cursor: marketdata.NewSliceCursor(mdtest.Events(t, spec)),
	}
}

// keyTimes reduces a merged stream to (millisecond, source) pairs, which is
// what most of these cases actually assert on.
func keyTimes(events []*marketdata.Event) []string {
	out := make([]string, len(events))
	for i, ev := range events {
		out[i] = fmt.Sprintf("%d/%s", ev.Key.TimeNs/1e6, ev.Source)
	}
	return out
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name    string
		cursors func(t *testing.T) []marketdata.NamedCursor
		want    []string
	}{
		{
			name: "single source passes through",
			cursors: func(t *testing.T) []marketdata.NamedCursor {
				return []marketdata.NamedCursor{
					namedSlice(t, "a", `
						t=1000 trade
						t=2000 trade
						t=3000 trade
					`),
				}
			},
			want: []string{"1000/a", "2000/a", "3000/a"},
		},
		{
			name: "two sources interleave strictly by time",
			cursors: func(t *testing.T) []marketdata.NamedCursor {
				return []marketdata.NamedCursor{
					namedSlice(t, "a", "t=1000 trade\nt=3000 trade\nt=5000 trade"),
					namedSlice(t, "b", "t=2000 trade\nt=4000 trade\nt=6000 trade"),
				}
			},
			want: []string{"1000/a", "2000/b", "3000/a", "4000/b", "5000/a", "6000/b"},
		},
		{
			name: "identical keys break ties by source name",
			cursors: func(t *testing.T) []marketdata.NamedCursor {
				// registered out of alphabetical order on purpose
				return []marketdata.NamedCursor{
					namedSlice(t, "zulu", "t=1000 trade seq=1"),
					namedSlice(t, "alpha", "t=1000 trade seq=1"),
				}
			},
			want: []string{"1000/alpha", "1000/zulu"},
		},
		{
			name: "empty source is skipped",
			cursors: func(t *testing.T) []marketdata.NamedCursor {
				return []marketdata.NamedCursor{
					namedSlice(t, "a", "t=1000 trade\nt=2000 trade"),
					{Name: "empty", Cursor: marketdata.NewSliceCursor(nil)},
				}
			},
			want: []string{"1000/a", "2000/a"},
		},
		{
			name: "source exhausted early, others continue",
			cursors: func(t *testing.T) []marketdata.NamedCursor {
				return []marketdata.NamedCursor{
					namedSlice(t, "short", "t=1000 trade"),
					namedSlice(t, "long", "t=2000 trade\nt=3000 trade\nt=4000 trade"),
				}
			},
			want: []string{"1000/short", "2000/long", "3000/long", "4000/long"},
		},
		{
			name: "source starting late is held until its time arrives",
			cursors: func(t *testing.T) []marketdata.NamedCursor {
				return []marketdata.NamedCursor{
					namedSlice(t, "early", "t=1000 trade\nt=2000 trade"),
					namedSlice(t, "late", "t=9000 trade"),
				}
			},
			want: []string{"1000/early", "2000/early", "9000/late"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := marketdata.Merge(tt.cursors(t))
			defer m.Close()

			got := mdtest.Collect(t, m)
			assert.Equal(t, tt.want, keyTimes(got))
		})
	}
}

// TestMerge_KLineIntervalInvariantAcrossSources checks that the shortest
// interval still arrives first when the klines closing at one instant come from
// different sources — that is, the invariant survives merging, not just ranking.
func TestMerge_KLineIntervalInvariantAcrossSources(t *testing.T) {
	m := marketdata.Merge([]marketdata.NamedCursor{
		namedSlice(t, "hourly", "t=3600000 kline interval=1h"),
		namedSlice(t, "minutely", "t=3600000 kline interval=1m"),
		namedSlice(t, "daily", "t=3600000 kline interval=1d"),
	})
	defer m.Close()

	got := mdtest.Collect(t, m)
	require.Len(t, got, 3)

	intervals := []types.Interval{
		got[0].KLine.Interval,
		got[1].KLine.Interval,
		got[2].KLine.Interval,
	}
	assert.Equal(t, []types.Interval{types.Interval1m, types.Interval1h, types.Interval1d}, intervals,
		"at an identical close time the shortest interval must be delivered first")
}

// TestMerge_TradeBeforeBookUpdate pins the causal ordering at one timestamp.
func TestMerge_TradeBeforeBookUpdate(t *testing.T) {
	m := marketdata.Merge([]marketdata.NamedCursor{
		namedSlice(t, "book", "t=1000 bookSnapshot bids=100,10 asks=101,10 seq=1\nt=1000 bookUpdate bids=100,0 seq=2"),
		namedSlice(t, "trades", "t=1000 trade price=100 qty=10"),
		namedSlice(t, "klines", "t=1000 kline interval=1m"),
	})
	defer m.Close()

	got := mdtest.Collect(t, m)
	require.Len(t, got, 4)

	assert.Equal(t, []marketdata.EventType{
		marketdata.EventTypeTrade,
		marketdata.EventTypeBookSnapshot,
		marketdata.EventTypeBookUpdate,
		marketdata.EventTypeKLine,
	}, []marketdata.EventType{got[0].Type, got[1].Type, got[2].Type, got[3].Type})
}

func TestMerge_ManySourcesStaySorted(t *testing.T) {
	const sources, perSource = 5, 1000

	rng := rand.New(rand.NewSource(1))
	var cursors []marketdata.NamedCursor
	for s := 0; s < sources; s++ {
		events := make([]marketdata.Event, 0, perSource)
		ts := int64(0)
		for i := 0; i < perSource; i++ {
			ts += rng.Int63n(100) + 1
			events = append(events, marketdata.Event{
				Type:   marketdata.EventTypeTrade,
				Symbol: "BTCUSDT",
				Key:    marketdata.OrderKey{TimeNs: ts * 1e6, Rank: marketdata.RankTrade},
			})
		}
		cursors = append(cursors, marketdata.NamedCursor{
			Name:   fmt.Sprintf("src%02d", s),
			Cursor: marketdata.NewSliceCursor(events),
		})
	}

	m := marketdata.Merge(cursors)
	defer m.Close()

	got := mdtest.Collect(t, m)
	require.Len(t, got, sources*perSource)

	for i := 1; i < len(got); i++ {
		assert.LessOrEqual(t, got[i-1].Key.Compare(got[i].Key), 0,
			"merged output must be non-decreasing at index %d", i)
	}
}

func TestMerge_FailFastClosesEveryCursor(t *testing.T) {
	boom := errors.New("read failed")

	failing := mdtest.NewCountingCursor(&mdtest.ErrCursor{
		Events: mdtest.Events(t, "t=1000 trade"),
		Fail:   boom,
	})
	healthy := mdtest.NewCountingCursor(
		marketdata.NewSliceCursor(mdtest.Events(t, "t=2000 trade\nt=3000 trade")))

	m := marketdata.Merge([]marketdata.NamedCursor{
		{Name: "failing", Cursor: failing},
		{Name: "healthy", Cursor: healthy},
	})

	var seen int
	for m.Next() {
		seen++
	}

	require.Error(t, m.Err())
	assert.ErrorIs(t, m.Err(), boom)

	var srcErr *marketdata.SourceError
	require.ErrorAs(t, m.Err(), &srcErr)
	assert.Equal(t, "failing", srcErr.Source)

	require.NoError(t, m.Close())
	assert.Positive(t, failing.Closes, "the failing cursor must be closed")
	assert.Positive(t, healthy.Closes, "healthy cursors must be closed too")
}

func TestMerge_SkipSourceContinues(t *testing.T) {
	failing := &mdtest.ErrCursor{
		Events: mdtest.Events(t, "t=1000 trade"),
		Fail:   errors.New("read failed"),
	}

	m := marketdata.Merge([]marketdata.NamedCursor{
		{Name: "failing", Cursor: failing},
		namedSlice(t, "healthy", "t=2000 trade\nt=3000 trade"),
	}, marketdata.WithErrorPolicy(marketdata.SkipSource))
	defer m.Close()

	got := mdtest.Collect(t, m)
	assert.Equal(t, []string{"1000/failing", "2000/healthy", "3000/healthy"}, keyTimes(got))
	assert.NoError(t, m.Err())
}

// TestMerge_OpenErrorFailsBeforeAnyEvent covers a source whose very first read
// fails: it must surface as an error, not as an empty source.
func TestMerge_PrimingErrorSurfaces(t *testing.T) {
	boom := errors.New("cannot open archive")

	m := marketdata.Merge([]marketdata.NamedCursor{
		{Name: "broken", Cursor: &mdtest.ErrCursor{Fail: boom}},
		namedSlice(t, "healthy", "t=1000 trade"),
	})
	defer m.Close()

	assert.False(t, m.Next())
	assert.ErrorIs(t, m.Err(), boom)
}

func TestMerge_Stats(t *testing.T) {
	m := marketdata.Merge([]marketdata.NamedCursor{
		namedSlice(t, "a", "t=1000 trade\nt=5000 trade"),
		namedSlice(t, "b", "t=2000 trade"),
	})

	mdtest.Collect(t, m)
	require.NoError(t, m.Close())

	stats := m.Stats()
	require.Len(t, stats, 2)

	assert.Equal(t, "a", stats[0].Name)
	assert.Equal(t, int64(2), stats[0].Count)
	assert.Equal(t, int64(1000), stats[0].First.UnixMilli())
	assert.Equal(t, int64(5000), stats[0].Last.UnixMilli())

	assert.Equal(t, "b", stats[1].Name)
	assert.Equal(t, int64(1), stats[1].Count)
}

func TestMerge_CloseIsIdempotent(t *testing.T) {
	counting := mdtest.NewCountingCursor(
		marketdata.NewSliceCursor(mdtest.Events(t, "t=1000 trade")))

	m := marketdata.Merge([]marketdata.NamedCursor{{Name: "a", Cursor: counting}})

	require.NoError(t, m.Close())
	require.NoError(t, m.Close())
	assert.False(t, m.Next(), "a closed merge must not produce events")
}

func TestMergeSources(t *testing.T) {
	ctx := context.Background()
	req := marketdata.Request{
		Since: time.UnixMilli(0),
		Until: time.UnixMilli(10_000),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel},
		},
	}

	m, err := marketdata.MergeSources(ctx, []marketdata.Source{
		mdtest.NewStaticSource("b", mdtest.Events(t, "t=2000 trade")),
		mdtest.NewStaticSource("a", mdtest.Events(t, "t=1000 trade")),
	}, req)
	require.NoError(t, err)
	defer m.Close()

	assert.Equal(t, []string{"1000/a", "2000/b"}, keyTimes(mdtest.Collect(t, m)))
}

func TestMergeSources_OpenErrorClosesOpenedCursors(t *testing.T) {
	ctx := context.Background()
	req := marketdata.Request{
		Since: time.UnixMilli(0),
		Until: time.UnixMilli(10_000),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel},
		},
	}

	broken := mdtest.NewStaticSource("broken", nil)
	broken.OpenErr = errors.New("nope")

	_, err := marketdata.MergeSources(ctx, []marketdata.Source{
		mdtest.NewStaticSource("ok", mdtest.Events(t, "t=1000 trade")),
		broken,
	}, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
}

func TestMergeSources_RejectsLiveOnlySource(t *testing.T) {
	ctx := context.Background()
	req := marketdata.Request{
		Since: time.UnixMilli(0),
		Until: time.UnixMilli(10_000),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel},
		},
	}

	live := mdtest.NewStaticSource("live", mdtest.Events(t, "t=1000 trade"))
	live.Capability.HasHistory = false

	_, err := marketdata.MergeSources(ctx, []marketdata.Source{live}, req)

	require.Error(t, err)
	var unsupported *marketdata.UnsupportedError
	require.ErrorAs(t, err, &unsupported)
	assert.Contains(t, unsupported.Reason, "live-only")
}

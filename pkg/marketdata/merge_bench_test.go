package marketdata_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
)

func benchCursors(sources, perSource int) []marketdata.NamedCursor {
	out := make([]marketdata.NamedCursor, 0, sources)
	for s := 0; s < sources; s++ {
		events := make([]marketdata.Event, perSource)
		for i := range events {
			events[i] = marketdata.Event{
				Type:   marketdata.EventTypeTrade,
				Symbol: "BTCUSDT",
				// interleave the sources so every step re-sifts the heap
				Key: marketdata.OrderKey{
					TimeNs: int64(i*sources+s) * 1e6,
					Rank:   marketdata.RankTrade,
				},
			}
		}
		out = append(out, marketdata.NamedCursor{
			Name:   fmt.Sprintf("src%02d", s),
			Cursor: marketdata.NewSortedSliceCursor(events),
		})
	}
	return out
}

// BenchmarkMerge8x100k is the allocation regression guard: the steady-state
// merge must not allocate per event. If allocs/op grows with the event count,
// something started copying or boxing events in the hot path.
func BenchmarkMerge8x100k(b *testing.B) {
	const sources, perSource = 8, 100_000

	for b.Loop() {
		m := marketdata.Merge(benchCursors(sources, perSource))
		var n int
		for m.Next() {
			n++
		}
		if n != sources*perSource {
			b.Fatalf("expected %d events, got %d", sources*perSource, n)
		}
		m.Close()
	}
}

// TestMerge_SteadyStateIsAllocationFree is the real regression guard, and it is
// a test rather than a benchmark so CI enforces it.
//
// Draining the merge must not allocate: the heap holds pointers to events the
// child cursors own, and Next only re-sifts. If this starts failing, something
// began copying or boxing events in the hot path.
func TestMerge_SteadyStateIsAllocationFree(t *testing.T) {
	const sources, perSource = 4, 512

	m := marketdata.Merge(benchCursors(sources, perSource))
	t.Cleanup(func() { m.Close() })

	// prime: the first Next touches the heap bookkeeping
	require.True(t, m.Next())

	allocs := testing.AllocsPerRun(1000, func() {
		m.Next()
	})

	assert.Zero(t, allocs, "MergeCursor.Next must not allocate in steady state")
}

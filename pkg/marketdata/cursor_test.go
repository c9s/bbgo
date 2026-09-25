package marketdata_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/mdtest"
)

func TestSliceCursor_SortsOnConstruction(t *testing.T) {
	// deliberately out of order
	c := marketdata.NewSliceCursor(mdtest.Events(t, `
		t=3000 trade
		t=1000 trade
		t=2000 trade
	`))

	var times []int64
	for c.Next() {
		times = append(times, c.Event().Key.TimeNs/1e6)
	}

	require.NoError(t, c.Err())
	assert.Equal(t, []int64{1000, 2000, 3000}, times)
}

func TestSliceCursor_ExhaustionAndClose(t *testing.T) {
	c := marketdata.NewSliceCursor(mdtest.Events(t, "t=1000 trade"))

	require.True(t, c.Next())
	assert.False(t, c.Next())
	assert.False(t, c.Next(), "Next must keep returning false after exhaustion")
	assert.Nil(t, c.Event())

	require.NoError(t, c.Close())
	require.NoError(t, c.Close(), "Close must be idempotent")
}

func TestSliceCursor_CloseBeforeNext(t *testing.T) {
	c := marketdata.NewSliceCursor(mdtest.Events(t, "t=1000 trade"))
	require.NoError(t, c.Close())
	assert.False(t, c.Next())
}

func TestValidatingCursor_CatchesOutOfOrder(t *testing.T) {
	// NewSortedSliceCursor skips the sort, so this cursor genuinely violates the
	// ordering contract and the wrapper must catch it.
	inner := marketdata.NewSortedSliceCursor(mdtest.Events(t, `
		t=3000 trade
		t=1000 trade
	`))

	c := marketdata.Validate(inner, "broken")

	require.True(t, c.Next())
	assert.False(t, c.Next())
	assert.ErrorIs(t, c.Err(), marketdata.ErrOutOfOrder)
}

func TestValidatingCursor_PassesSortedStream(t *testing.T) {
	c := marketdata.Validate(
		marketdata.NewSliceCursor(mdtest.Events(t, "t=1000 trade\nt=2000 trade")), "good")

	assert.Len(t, mdtest.Collect(t, c), 2)
	assert.NoError(t, c.Err())
}

// TestValidatingCursor_IgnoresSourceIndex checks that the wrapper compares keys
// with SourceIndex masked out, since that field is the merge's to assign.
func TestValidatingCursor_IgnoresSourceIndex(t *testing.T) {
	events := mdtest.Events(t, "t=1000 trade\nt=1000 trade")
	events[0].Key.SourceIndex = 5
	events[1].Key.SourceIndex = 1

	c := marketdata.Validate(marketdata.NewSortedSliceCursor(events), "same-time")

	assert.Len(t, mdtest.Collect(t, c), 2)
	assert.NoError(t, c.Err())
}

func TestSeq(t *testing.T) {
	c := marketdata.NewSliceCursor(mdtest.Events(t, "t=1000 trade\nt=2000 trade\nt=3000 trade"))

	var times []int64
	for ev := range marketdata.Seq(c) {
		times = append(times, ev.Key.TimeNs/1e6)
	}

	require.NoError(t, c.Err())
	assert.Equal(t, []int64{1000, 2000, 3000}, times)
}

func TestSeq_EarlyBreak(t *testing.T) {
	c := marketdata.NewSliceCursor(mdtest.Events(t, "t=1000 trade\nt=2000 trade"))

	var count int
	for range marketdata.Seq(c) {
		count++
		break
	}

	assert.Equal(t, 1, count)
	require.NoError(t, c.Close())
}

func TestEvent_Clone(t *testing.T) {
	events := mdtest.Events(t, "t=1000 bookSnapshot seq=1 bids=100,10;99,5 asks=101,10")
	orig := &events[0]

	clone := orig.Clone()
	require.NotNil(t, clone.Book)
	assert.NotSame(t, orig.Book, clone.Book)

	// mutating the clone's levels must not touch the original
	clone.Book.Bids[0].Volume = clone.Book.Bids[0].Volume.Mul(clone.Book.Bids[0].Volume)
	assert.Equal(t, "10", orig.Book.Bids[0].Volume.String())
}

func TestEvent_TimeIsUTC(t *testing.T) {
	ev := marketdata.Event{Key: marketdata.OrderKey{TimeNs: 1_700_000_000_123_456_789}}
	got := ev.Time()

	assert.Equal(t, "UTC", got.Location().String())
	assert.Equal(t, 123456789, got.Nanosecond(), "nanosecond precision must survive")
}

func TestEventFlag_Has(t *testing.T) {
	f := marketdata.FlagSynthetic | marketdata.FlagPartialDepth

	assert.True(t, f.Has(marketdata.FlagSynthetic))
	assert.True(t, f.Has(marketdata.FlagPartialDepth))
	assert.False(t, f.Has(marketdata.FlagAggregated))
}

package types

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func newTestStreamBookMonitor() (*StreamBookMonitor, *StreamOrderBook) {
	m := &StreamBookMonitor{
		Enabled:        true,
		StaleThreshold: Duration(5 * time.Second),
	}

	book := NewStreamBook("BTCUSDT", ExchangeBinance)
	m.Bind(book)

	return m, book
}

// validOrderBook returns a SliceOrderBook with a non-empty, non-crossed
// bid/ask, i.e. one that passes SliceOrderBook.IsValid().
func validOrderBook(symbol string, t time.Time) SliceOrderBook {
	return SliceOrderBook{
		Symbol: symbol,
		Time:   t,
		Bids:   PriceVolumeSlice{{Price: fixedpoint.NewFromInt(100), Volume: fixedpoint.NewFromInt(1)}},
		Asks:   PriceVolumeSlice{{Price: fixedpoint.NewFromInt(101), Volume: fixedpoint.NewFromInt(1)}},
	}
}

func TestStreamBookMonitor_Defaults(t *testing.T) {
	m := &StreamBookMonitor{Enabled: true}
	assert.NoError(t, m.Defaults())
	assert.Equal(t, 5*time.Second, m.StaleThreshold.Duration())
}

func TestStreamBookMonitor_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		m := &StreamBookMonitor{Enabled: true, StaleThreshold: Duration(5 * time.Second)}
		assert.NoError(t, m.Validate())
	})

	t.Run("non-positive threshold", func(t *testing.T) {
		m := &StreamBookMonitor{Enabled: true}
		assert.Error(t, m.Validate())

		m.StaleThreshold = Duration(-time.Second)
		assert.Error(t, m.Validate())
	})
}

func TestStreamBookMonitor_IsStale(t *testing.T) {
	m, _ := newTestStreamBookMonitor()
	now := time.Now()

	t.Run("no event observed yet", func(t *testing.T) {
		assert.True(t, m.IsStale(now), "a book that hasn't received its first snapshot is stale by definition")
	})

	t.Run("recent event within threshold", func(t *testing.T) {
		m.lastEventTime = now
		assert.False(t, m.IsStale(now.Add(time.Second)))
	})

	t.Run("long gap since last event", func(t *testing.T) {
		m.lastEventTime = now
		assert.True(t, m.IsStale(now.Add(10*time.Second)))
	})
}

func TestStreamBookMonitor_recordEvent(t *testing.T) {
	m, book := newTestStreamBookMonitor()

	assert.True(t, m.lastEventTime.IsZero())
	assert.True(t, m.IsStale(time.Now()))

	book.EmitUpdate(validOrderBook(book.Symbol, time.Now()))
	assert.False(t, m.lastEventTime.IsZero())
	assert.False(t, m.IsStale(time.Now()))

	firstEventTime := m.lastEventTime
	book.EmitUpdate(validOrderBook(book.Symbol, time.Now()))
	assert.True(t, m.lastEventTime.After(firstEventTime))
}

func TestStreamBookMonitor_recordEvent_usesServerTime(t *testing.T) {
	m, book := newTestStreamBookMonitor()

	serverTime := time.Now().Add(-time.Minute)
	book.EmitUpdate(validOrderBook(book.Symbol, serverTime))
	assert.True(t, m.lastEventTime.Equal(serverTime), "lastEventTime should track the event's own server time")
}

func TestStreamBookMonitor_recordEvent_fallsBackToNowWhenServerTimeIsZero(t *testing.T) {
	m, book := newTestStreamBookMonitor()

	before := time.Now()
	book.EmitUpdate(validOrderBook(book.Symbol, time.Time{}))
	after := time.Now()

	assert.False(t, m.lastEventTime.Before(before))
	assert.False(t, m.lastEventTime.After(after))
}

func TestStreamBookMonitor_recordEvent_ignoresInvalidUpdate(t *testing.T) {
	t.Run("empty sides", func(t *testing.T) {
		m, book := newTestStreamBookMonitor()

		book.EmitUpdate(SliceOrderBook{Symbol: book.Symbol, Time: time.Now()})
		assert.True(t, m.lastEventTime.IsZero(), "an update with no bids/asks must not count as a fresh event")
	})

	t.Run("crossed book", func(t *testing.T) {
		m, book := newTestStreamBookMonitor()

		book.EmitUpdate(SliceOrderBook{
			Symbol: book.Symbol,
			Time:   time.Now(),
			Bids:   PriceVolumeSlice{{Price: fixedpoint.NewFromInt(200), Volume: fixedpoint.NewFromInt(1)}},
			Asks:   PriceVolumeSlice{{Price: fixedpoint.NewFromInt(100), Volume: fixedpoint.NewFromInt(1)}},
		})
		assert.True(t, m.lastEventTime.IsZero(), "a crossed bid/ask must not count as a fresh event")
	})

	t.Run("valid update following an invalid one still records", func(t *testing.T) {
		m, book := newTestStreamBookMonitor()

		book.EmitUpdate(SliceOrderBook{Symbol: book.Symbol, Time: time.Now()})
		assert.True(t, m.lastEventTime.IsZero())

		book.EmitUpdate(validOrderBook(book.Symbol, time.Now()))
		assert.False(t, m.lastEventTime.IsZero())
	})
}

// fakeDepthQueryService is a minimal ExchangeDepthQueryService test double.
type fakeDepthQueryService struct {
	snapshot SliceOrderBook
	err      error
}

func (f *fakeDepthQueryService) QueryDepth(_ context.Context, _ string) (SliceOrderBook, int64, error) {
	return f.snapshot, 0, f.err
}

func TestStreamBookMonitor_PollUpdate(t *testing.T) {
	m, book := newTestStreamBookMonitor()

	snapshotTime := time.Now()
	query := &fakeDepthQueryService{
		snapshot: SliceOrderBook{
			Symbol: book.Symbol,
			Time:   snapshotTime,
			Bids:   PriceVolumeSlice{{Price: fixedpoint.NewFromInt(100), Volume: fixedpoint.NewFromInt(1)}},
			Asks:   PriceVolumeSlice{{Price: fixedpoint.NewFromInt(101), Volume: fixedpoint.NewFromInt(1)}},
		},
	}

	err := m.PollUpdate(context.Background(), query)
	assert.NoError(t, err)

	bid, ask, ok := book.BestBidAndAsk()
	assert.True(t, ok)
	assert.True(t, fixedpoint.NewFromInt(100).Eq(bid.Price))
	assert.True(t, fixedpoint.NewFromInt(101).Eq(ask.Price))

	// a poll must not be mistaken for a stream update/snapshot event
	assert.True(t, m.lastEventTime.IsZero())
}

func TestStreamBookMonitor_PollUpdate_error(t *testing.T) {
	m, _ := newTestStreamBookMonitor()

	query := &fakeDepthQueryService{err: assert.AnError}
	err := m.PollUpdate(context.Background(), query)
	assert.Error(t, err)
}

package marketdata_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/mdtest"
	"github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestBookState_SnapshotThenUpdate(t *testing.T) {
	s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)

	events := mdtest.Events(t, `
		t=1000 bookSnapshot seq=100 bids=100,10;99,5 asks=101,10;102,5
		t=2000 bookUpdate   seq=101 bids=100,20
	`)

	for i := range events {
		require.NoError(t, s.Apply(&events[i]))
	}

	bid, ok := s.Book.BestBid()
	require.True(t, ok)
	assert.Equal(t, "100", bid.Price.String())
	assert.Equal(t, "20", bid.Volume.String(), "the update must replace the level volume")

	ask, ok := s.Book.BestAsk()
	require.True(t, ok)
	assert.Equal(t, "101", ask.Price.String())
}

// TestBookState_ZeroVolumeRemovesLevel pins the convention the whole L2 design
// leans on, and which Binance and AmberData share.
func TestBookState_ZeroVolumeRemovesLevel(t *testing.T) {
	s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)

	events := mdtest.Events(t, `
		t=1000 bookSnapshot seq=100 bids=100,10;99,5 asks=101,10
		t=2000 bookUpdate   seq=101 bids=100,0
	`)
	for i := range events {
		require.NoError(t, s.Apply(&events[i]))
	}

	bid, ok := s.Book.BestBid()
	require.True(t, ok)
	assert.Equal(t, "99", bid.Price.String(),
		"a zero-volume update must remove the level, promoting the next one")
}

func TestBookState_UpdateBeforeSnapshot(t *testing.T) {
	s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)

	events := mdtest.Events(t, "t=1000 bookUpdate seq=101 bids=100,10")
	err := s.Apply(&events[0])

	assert.ErrorIs(t, err, marketdata.ErrBookNotReady)
	assert.False(t, s.Ready())
}

func TestBookState_SequenceModes(t *testing.T) {
	// seq jumps from 100 to 105: a hole of four updates.
	spec := `
		t=1000 bookSnapshot seq=100 bids=100,10 asks=101,10
		t=2000 bookUpdate   seq=105 bids=100,20
	`

	t.Run("none tolerates the gap", func(t *testing.T) {
		s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
		s.Mode = marketdata.SequenceNone

		events := mdtest.Events(t, spec)
		for i := range events {
			require.NoError(t, s.Apply(&events[i]))
		}
	})

	t.Run("monotonic reports but tolerates", func(t *testing.T) {
		s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
		s.Mode = marketdata.SequenceMonotonic

		var gotExpected, gotActual uint64
		s.OnGap = func(expected, got uint64) { gotExpected, gotActual = expected, got }

		events := mdtest.Events(t, spec)
		for i := range events {
			require.NoError(t, s.Apply(&events[i]))
		}

		assert.Equal(t, uint64(101), gotExpected)
		assert.Equal(t, uint64(105), gotActual)
	})

	t.Run("contiguous errors on the gap", func(t *testing.T) {
		s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
		s.Mode = marketdata.SequenceContiguous

		events := mdtest.Events(t, spec)
		require.NoError(t, s.Apply(&events[0]))
		assert.ErrorIs(t, s.Apply(&events[1]), marketdata.ErrBookGap)
	})

	t.Run("contiguous accepts consecutive sequences", func(t *testing.T) {
		s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
		s.Mode = marketdata.SequenceContiguous

		events := mdtest.Events(t, `
			t=1000 bookSnapshot seq=100 bids=100,10 asks=101,10
			t=2000 bookUpdate   seq=101 bids=100,20
			t=3000 bookUpdate   seq=102 bids=100,30
		`)
		for i := range events {
			require.NoError(t, s.Apply(&events[i]))
		}
	})
}

func TestBookState_SequenceGoesBackwards(t *testing.T) {
	s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
	s.Mode = marketdata.SequenceMonotonic

	events := mdtest.Events(t, `
		t=1000 bookSnapshot seq=100 bids=100,10 asks=101,10
		t=2000 bookUpdate   seq=99  bids=100,20
	`)

	require.NoError(t, s.Apply(&events[0]))
	assert.ErrorIs(t, s.Apply(&events[1]), marketdata.ErrBookGap)
}

func TestBookState_IgnoresNonBookEvents(t *testing.T) {
	s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)

	events := mdtest.Events(t, "t=1000 trade\nt=2000 kline interval=1m")
	for i := range events {
		require.NoError(t, s.Apply(&events[i]),
			"a consumer must be able to feed the whole merged stream to Apply")
	}
	assert.False(t, s.Ready())
}

// TestBookState_CheckCrossed shows how a corrupted diff stream is caught rather
// than silently producing wrong fills.
func TestBookState_CheckCrossed(t *testing.T) {
	s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
	s.CheckCrossed = true

	events := mdtest.Events(t, `
		t=1000 bookSnapshot seq=100 bids=100,10 asks=101,10
		t=2000 bookUpdate   seq=101 bids=102,10
	`)

	require.NoError(t, s.Apply(&events[0]))
	assert.ErrorIs(t, s.Apply(&events[1]), marketdata.ErrBookCrossed,
		"a bid above the best ask must be reported")
}

func TestBookState_Reset(t *testing.T) {
	s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)

	events := mdtest.Events(t, "t=1000 bookSnapshot seq=100 bids=100,10 asks=101,10")
	require.NoError(t, s.Apply(&events[0]))
	require.True(t, s.Ready())

	s.Reset()
	assert.False(t, s.Ready())
	assert.Zero(t, s.LastSequence())
}

// TestBookState_AcceptsTesthelperLevels wires the repo's existing price/volume
// text helper into a snapshot, so provider tests can build fixtures the same way.
func TestBookState_AcceptsTesthelperLevels(t *testing.T) {
	s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)

	ev := marketdata.Event{
		Type:     marketdata.EventTypeBookSnapshot,
		Exchange: types.ExchangeBinance,
		Symbol:   "BTCUSDT",
		Key:      marketdata.OrderKey{TimeNs: 1e9, Rank: marketdata.RankBookSnapshot, Seq: 1},
		Book: &types.SliceOrderBook{
			Symbol: "BTCUSDT",
			Bids:   testhelper.PriceVolumeSliceFromText("100, 10\n99, 20"),
			Asks:   testhelper.PriceVolumeSliceFromText("101, 10\n102, 20"),
		},
	}

	require.NoError(t, s.Apply(&ev))

	bid, ask, ok := s.Book.BestBidAndAsk()
	require.True(t, ok)
	assert.Equal(t, "100", bid.Price.String())
	assert.Equal(t, "101", ask.Price.String())
}

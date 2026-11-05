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

// TestBookState_SequenceModes covers the corrected contiguity model.
//
// A venue sequence is not a counter. Binance's "u" is the id of the last
// individual update inside a batched diff event, so consecutive events differ by
// however many updates they carried — a real capture jumps from 100529710516 to
// 100529710641 with nothing missing. Contiguity therefore has to be checked
// against the previous id the venue names (Binance's "pu"), which the event
// carries as PrevSeq.
func TestBookState_SequenceModes(t *testing.T) {
	// seq jumps from 100 to 105, which on its own says nothing
	spec := `
		t=1000 bookSnapshot seq=100 bids=100,10 asks=101,10
		t=2000 bookUpdate   seq=105 bids=100,20
	`

	t.Run("none accepts anything forward", func(t *testing.T) {
		s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
		s.Mode = marketdata.SequenceNone

		events := mdtest.Events(t, spec)
		for i := range events {
			require.NoError(t, s.Apply(&events[i]))
		}
	})

	t.Run("monotonic accepts a forward jump", func(t *testing.T) {
		s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
		s.Mode = marketdata.SequenceMonotonic

		events := mdtest.Events(t, spec)
		for i := range events {
			require.NoError(t, s.Apply(&events[i]),
				"a jump in a venue sequence is not evidence of a lost update")
		}
	})

	t.Run("contiguous accepts a chained update", func(t *testing.T) {
		s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
		s.Mode = marketdata.SequenceContiguous

		events := mdtest.Events(t, spec)
		events[1].PrevSeq = 100 // the update says it follows the snapshot

		for i := range events {
			require.NoError(t, s.Apply(&events[i]))
		}
		assert.Zero(t, s.Unverified())
	})

	t.Run("contiguous rejects a broken chain", func(t *testing.T) {
		s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
		s.Mode = marketdata.SequenceContiguous

		var haveSeq, claimedSeq uint64
		s.OnGap = func(have, claimed uint64) { haveSeq, claimedSeq = have, claimed }

		events := mdtest.Events(t, spec)
		events[1].PrevSeq = 104 // claims to follow an update we never saw

		require.NoError(t, s.Apply(&events[0]))
		err := s.Apply(&events[1])

		require.ErrorIs(t, err, marketdata.ErrBookGap)
		assert.Contains(t, err.Error(), "claims to follow 104")
		assert.Equal(t, uint64(100), haveSeq)
		assert.Equal(t, uint64(104), claimedSeq)
	})

	// This is the case a live Binance recording actually produces today: the
	// venue's "pu" is not exposed through types.Stream, so the recorder cannot
	// capture it. Rather than inventing a rule and reporting false gaps, the
	// state counts what it could not verify.
	t.Run("contiguous counts what it cannot verify", func(t *testing.T) {
		s := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
		s.Mode = marketdata.SequenceContiguous

		events := mdtest.Events(t, `
			t=1000 bookSnapshot seq=100529710516 bids=100,10 asks=101,10
			t=2000 bookUpdate   seq=100529710641 bids=100,20
			t=3000 bookUpdate   seq=100529710890 bids=100,30
		`)
		for i := range events {
			require.NoError(t, s.Apply(&events[i]),
				"real venue sequences jump; that is not a gap")
		}

		assert.Equal(t, int64(2), s.Unverified(),
			"the two unprovable updates must be counted, not silently accepted")
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
	err := s.Apply(&events[1])
	require.ErrorIs(t, err, marketdata.ErrBookGap)
	assert.Contains(t, err.Error(), "went backwards")
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

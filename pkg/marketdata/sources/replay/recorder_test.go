package replay

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/mdtest"
	"github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

// freezeClock makes the arrival-time fallback deterministic, so a recording of
// events whose venue timestamp is missing is reproducible.
func freezeClock(t *testing.T, at time.Time) {
	t.Helper()

	prev := nowNs
	nowNs = func() int64 { return at.UnixNano() }
	t.Cleanup(func() { nowNs = prev })
}

// TestRecorder_BindStream records off a real types.StandardStream, which is what
// every exchange adapter emits through, so the recorder is exercised the same
// way a live capture would exercise it.
func TestRecorder_BindStream(t *testing.T) {
	dir := t.TempDir()
	w := testWriter(t, dir)

	base := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	freezeClock(t, base)

	stream := &types.StandardStream{}
	recorder := NewRecorder(w, testExchange)
	recorder.BindStream(stream)

	stream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol:       "BTCUSDT",
		Bids:         testhelper.PriceVolumeSliceFromText("100, 10\n99, 5"),
		Asks:         testhelper.PriceVolumeSliceFromText("101, 10\n102, 20"),
		Time:         base,
		LastUpdateId: 500,
	})

	stream.EmitBookUpdate(types.SliceOrderBook{
		Symbol:       "BTCUSDT",
		Bids:         testhelper.PriceVolumeSliceFromText("100, 0"),
		Time:         base.Add(time.Second),
		LastUpdateId: 501,
	})

	stream.EmitMarketTrade(types.Trade{
		ID:       9001,
		Symbol:   "BTCUSDT",
		Price:    testhelper.Number("100.5"),
		Quantity: testhelper.Number("0.5"),
		Side:     types.SideTypeBuy,
		IsBuyer:  true,
		Time:     types.Time(base.Add(2 * time.Second)),
	})

	stream.EmitKLineClosed(types.KLine{
		Symbol:    "BTCUSDT",
		Interval:  types.Interval1m,
		StartTime: types.Time(base),
		EndTime:   types.Time(base.Add(time.Minute).Add(-time.Millisecond)),
		Open:      testhelper.Number("100"),
		High:      testhelper.Number("102"),
		Low:       testhelper.Number("99"),
		Close:     testhelper.Number("101"),
		Volume:    testhelper.Number("12"),
		Closed:    true,
	})

	require.NoError(t, w.Close())
	assert.Zero(t, recorder.Dropped())
	assert.Equal(t, int64(4), w.Count())

	src, err := New(Config{Path: dir})
	require.NoError(t, err)

	cur, err := src.Open(context.Background(), fullRange())
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 4)

	assert.Equal(t, marketdata.EventTypeBookSnapshot, got[0].Type)
	assert.Equal(t, uint64(500), got[0].Key.Seq,
		"the snapshot's LastUpdateId must be recorded as the sequence")

	assert.Equal(t, marketdata.EventTypeBookUpdate, got[1].Type)
	assert.Equal(t, uint64(501), got[1].Key.Seq,
		"a diff's LastUpdateId must survive; without the binance stream fix this was 0")

	assert.Equal(t, marketdata.EventTypeTrade, got[2].Type)
	assert.Equal(t, uint64(9001), got[2].Key.Seq)

	assert.Equal(t, marketdata.EventTypeKLine, got[3].Type)
	assert.Equal(t, marketdata.KLineRank(types.Interval1m), got[3].Key.Rank)
}

// TestRecorder_BookWithoutVenueTimeFallsBackToArrival documents the fallback:
// an adapter that leaves SliceOrderBook.Time empty would otherwise record a
// zero timestamp, which sorts before everything else in a merge.
func TestRecorder_BookWithoutVenueTimeFallsBackToArrival(t *testing.T) {
	dir := t.TempDir()
	w := testWriter(t, dir)

	arrival := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	freezeClock(t, arrival)

	stream := &types.StandardStream{}
	NewRecorder(w, testExchange).BindStream(stream)

	stream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol:       "BTCUSDT",
		Bids:         testhelper.PriceVolumeSliceFromText("100, 10"),
		Asks:         testhelper.PriceVolumeSliceFromText("101, 10"),
		LastUpdateId: 1,
		// Time deliberately left zero
	})
	require.NoError(t, w.Close())

	src, err := New(Config{Path: dir})
	require.NoError(t, err)

	cur, err := src.Open(context.Background(), fullRange())
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 1)
	assert.Equal(t, arrival.UnixNano(), got[0].Key.TimeNs)
}

// TestL2ReplayAppliesCleanly is the end-to-end proof for L2, and the reason this
// provider exists: a recorded snapshot plus diffs replays into a book with no
// sequence gap, under the strictest validation mode.
//
// Neither data.binance.vision nor an unauthenticated vendor API can supply the
// input for this, so without a recorder the L2 path could not be tested at all.
func TestL2ReplayAppliesCleanly(t *testing.T) {
	dir := t.TempDir()
	w := testWriter(t, dir)

	base := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	freezeClock(t, base)

	stream := &types.StandardStream{}
	NewRecorder(w, testExchange).BindStream(stream)

	stream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol:       "BTCUSDT",
		Bids:         testhelper.PriceVolumeSliceFromText("100, 10\n99, 20\n98, 30"),
		Asks:         testhelper.PriceVolumeSliceFromText("101, 10\n102, 20\n103, 30"),
		Time:         base,
		LastUpdateId: 1000,
	})

	// A realistic diff sequence: a resize, a removal, a new level, and a
	// crossing-side move.
	updates := []struct {
		bids, asks string
		seq        int64
	}{
		{bids: "100, 15", seq: 1001},
		{bids: "99, 0", seq: 1002},
		{bids: "99.5, 12", seq: 1003},
		{asks: "101, 0", seq: 1004},
		{asks: "100.5, 8", seq: 1005},
	}

	for i, u := range updates {
		stream.EmitBookUpdate(types.SliceOrderBook{
			Symbol:       "BTCUSDT",
			Bids:         testhelper.PriceVolumeSliceFromText(u.bids),
			Asks:         testhelper.PriceVolumeSliceFromText(u.asks),
			Time:         base.Add(time.Duration(i+1) * time.Second),
			LastUpdateId: u.seq,
		})
	}

	require.NoError(t, w.Close())

	src, err := New(Config{Path: dir})
	require.NoError(t, err)
	assert.Contains(t, src.Capabilities().Channels, types.BookChannel,
		"a recording is the one source in this repo that can serve L2")

	cur, err := src.Open(context.Background(), fullRange())
	require.NoError(t, err)
	defer cur.Close()

	book := marketdata.NewBookState("BTCUSDT", testExchange)
	book.Mode = marketdata.SequenceContiguous
	book.CheckCrossed = true

	var applied int
	for cur.Next() {
		require.NoError(t, book.Apply(cur.Event()),
			"replaying a recording must reproduce the book without a gap")
		applied++
	}
	require.NoError(t, cur.Err())

	assert.Equal(t, 6, applied)
	assert.Equal(t, uint64(1005), book.LastSequence())

	bid, ask, ok := book.Book.BestBidAndAsk()
	require.True(t, ok)
	assert.Equal(t, "100", bid.Price.String())
	assert.Equal(t, "15", bid.Volume.String(), "the resize must have been applied")
	assert.Equal(t, "100.5", ask.Price.String(),
		"the removal of 101 must promote the newly added 100.5")

	bids := book.Book.SideBook(types.SideTypeBuy)
	for _, pv := range bids {
		assert.NotEqual(t, "99", pv.Price.String(),
			"the zero-volume update must have removed the 99 level")
	}
}

// TestL2ReplayMergesWithTrades is the multi-source case the layer is for:
// recorded L2 interleaved with archive-style trades, in one ordered stream.
func TestL2ReplayMergesWithTrades(t *testing.T) {
	dir := t.TempDir()
	w := testWriter(t, dir)

	base := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	freezeClock(t, base)

	stream := &types.StandardStream{}
	NewRecorder(w, testExchange).BindStream(stream)

	stream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   testhelper.PriceVolumeSliceFromText("100, 10"),
		Asks:   testhelper.PriceVolumeSliceFromText("101, 10"),
		Time:   base, LastUpdateId: 1000,
	})
	for i := 1; i <= 3; i++ {
		stream.EmitBookUpdate(types.SliceOrderBook{
			Symbol:       "BTCUSDT",
			Bids:         testhelper.PriceVolumeSliceFromText("100, 11"),
			Time:         base.Add(time.Duration(i) * 2 * time.Second),
			LastUpdateId: int64(1000 + i),
		})
	}
	require.NoError(t, w.Close())

	bookSource, err := New(Config{Name: "book", Path: dir})
	require.NoError(t, err)

	// A trade every second, so it interleaves with the two-second book updates.
	var tradeEvents []marketdata.Event
	for i := 0; i < 6; i++ {
		ts := base.Add(time.Duration(i) * time.Second)
		tradeEvents = append(tradeEvents, marketdata.Event{
			Type:     marketdata.EventTypeTrade,
			Exchange: testExchange,
			Symbol:   "BTCUSDT",
			Key: marketdata.OrderKey{
				TimeNs: ts.UnixNano(), Rank: marketdata.RankTrade, Seq: uint64(i + 1),
			},
			Trade: &types.Trade{
				ID: uint64(i + 1), Symbol: "BTCUSDT",
				Price: testhelper.Number("100.5"), Quantity: testhelper.Number("1"),
				Time: types.Time(ts),
			},
		})
	}

	req := marketdata.Request{
		Since: base,
		Until: base.Add(time.Minute),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.BookChannel},
			{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel},
		},
	}

	merged, err := marketdata.MergeSources(context.Background(),
		[]marketdata.Source{bookSource, mdtest.NewStaticSource("trades", tradeEvents)}, req)
	require.NoError(t, err)
	defer merged.Close()

	book := marketdata.NewBookState("BTCUSDT", testExchange)
	book.Mode = marketdata.SequenceContiguous

	var prev marketdata.OrderKey
	var trades, books int
	for merged.Next() {
		ev := merged.Event()
		assert.LessOrEqual(t, prev.Compare(ev.Key), 0, "the merged stream must be ordered")
		prev = ev.Key

		switch ev.Type {
		case marketdata.EventTypeTrade:
			trades++
		default:
			require.NoError(t, book.Apply(ev))
			books++
		}
	}
	require.NoError(t, merged.Err())

	assert.Equal(t, 6, trades)
	assert.Equal(t, 4, books)
	assert.Equal(t, uint64(1003), book.LastSequence())
}

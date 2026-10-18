package replay

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/mdtest"
	"github.com/c9s/bbgo/pkg/types"
)

// TestLiveCaptureFixture replays a real Binance capture.
//
// This is the only real order book data in the repository — no public archive
// publishes L2 — so it is what keeps the format honest against what a venue
// actually sends, rather than against something hand-written to match the
// decoder.
func TestLiveCaptureFixture(t *testing.T) {
	src, err := New(Config{Path: "testdata/binance-BTCUSDT-20260923T03.jsonl"})
	require.NoError(t, err)

	assert.Contains(t, src.Capabilities().Channels, types.BookChannel)
	assert.Equal(t, []types.ExchangeName{types.ExchangeBinance}, src.Capabilities().Exchanges)

	req := marketdata.Request{
		Since: time.Unix(0, 0),
		Until: time.Now().AddDate(10, 0, 0),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.BookChannel},
			{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel},
		},
	}

	cur, err := src.Open(context.Background(), req)
	require.NoError(t, err)
	defer cur.Close()

	book := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
	book.Mode = marketdata.SequenceContiguous

	var (
		snapshots, updates, trades int
		prev                       marketdata.OrderKey
	)

	for cur.Next() {
		ev := cur.Event()

		assert.LessOrEqual(t, prev.Compare(ev.Key), 0, "a capture must replay in order")
		prev = ev.Key

		switch ev.Type {
		case marketdata.EventTypeBookSnapshot:
			snapshots++
		case marketdata.EventTypeBookUpdate:
			updates++
		case marketdata.EventTypeTrade:
			trades++
		}

		require.NoError(t, book.Apply(ev))
	}
	require.NoError(t, cur.Err())

	assert.Equal(t, 1, snapshots)
	assert.Equal(t, 5, updates)
	assert.Equal(t, 5, trades)

	// The sequences are the venue's own. Before the fix to
	// pkg/exchange/binance/stream.go every diff recorded a zero here.
	assert.Equal(t, uint64(100529710516), firstBookSeq(t),
		"the snapshot's LastUpdateId must be the venue's value")
	assert.Greater(t, book.LastSequence(), uint64(100529710516),
		"diffs must carry real, advancing sequences")

	// Binance batches individual updates into one event, so consecutive events
	// jump. That is not a gap, and the capture proves it is the normal case.
	assert.Equal(t, int64(5), book.Unverified(),
		"without the venue's previous-update id, contiguity is unprovable and must be counted")

	bid, ask, ok := book.Book.BestBidAndAsk()
	require.True(t, ok)
	assert.True(t, bid.Price.Compare(ask.Price) < 0, "the reconstructed book must not be crossed")
}

// firstBookSeq reads the first book record's sequence straight from the fixture,
// independently of the decode path being tested.
func firstBookSeq(t *testing.T) uint64 {
	t.Helper()

	src, err := New(Config{Path: "testdata/binance-BTCUSDT-20260923T03.jsonl"})
	require.NoError(t, err)

	cur, err := src.Open(context.Background(), marketdata.Request{
		Since: time.Unix(0, 0),
		Until: time.Now().AddDate(10, 0, 0),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.BookChannel},
		},
	})
	require.NoError(t, err)
	defer cur.Close()

	events := mdtest.Collect(t, cur)
	require.NotEmpty(t, events)
	return events[0].Key.Seq
}

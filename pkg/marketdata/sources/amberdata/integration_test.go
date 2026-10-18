package amberdata_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/mdtest"
	"github.com/c9s/bbgo/pkg/marketdata/sources/amberdata"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

// TestIntegration_Live is the only test here that talks to the real API, and it
// is the test that will settle the open questions in the package documentation.
//
// It is skipped unless AMBERDATA_API_KEY and TEST_AMBERDATA=1 are set, because
// the API has no free tier: there is no credential anyone can put in CI.
//
// Whoever has a key should also record the interactions, so everyone else can
// replay them:
//
//	AMBERDATA_API_KEY=... TEST_AMBERDATA=1 TEST_HTTP_RECORD=1 \
//	    go test ./pkg/marketdata/sources/amberdata/ -run TestIntegration_Live -v
//
// httptesting.Recorder strips credential headers before saving.
func TestIntegration_Live(t *testing.T) {
	apiKey, ok := testutil.APIKeyConfigured(t, "AMBERDATA")
	if !ok {
		t.Skip("set AMBERDATA_API_KEY and TEST_AMBERDATA=1 to run this")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	src, err := amberdata.New(amberdata.Config{
		APIKey:   apiKey,
		Exchange: "binance",
		Symbols:  []string{"BTCUSDT"},
		MaxLevel: 20,
		Chunk:    5 * time.Minute,
	})
	require.NoError(t, err)

	// A short, recent window: enough to exercise pagination without spending
	// much of the key's daily quota.
	until := time.Now().UTC().Add(-time.Hour).Truncate(time.Minute)
	since := until.Add(-10 * time.Minute)

	t.Run("trades", func(t *testing.T) {
		cur, err := src.Open(ctx, marketdata.Request{
			Since: since, Until: until,
			Subscriptions: []types.Subscription{
				{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel},
			},
		})
		require.NoError(t, err)
		defer cur.Close()

		events := mdtest.Collect(t, cur)
		require.NotEmpty(t, events, "a ten minute window of BTCUSDT should have trades")

		for i := 1; i < len(events); i++ {
			assert.LessOrEqual(t, events[i-1].Key.Compare(events[i].Key), 0)
		}

		t.Logf("%d trades, %s .. %s", len(events),
			events[0].Time().Format(time.RFC3339Nano),
			events[len(events)-1].Time().Format(time.RFC3339Nano))
	})

	// This subtest is what answers the biggest open question: whether the events
	// endpoint yields a book that can be reconstructed without gaps.
	t.Run("book", func(t *testing.T) {
		cur, err := src.Open(ctx, marketdata.Request{
			Since: since, Until: until,
			Subscriptions: []types.Subscription{
				{Symbol: "BTCUSDT", Channel: types.BookChannel},
			},
		})
		require.NoError(t, err)
		defer cur.Close()

		book := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
		book.Mode = marketdata.SequenceMonotonic
		book.CheckCrossed = true

		var snapshots, updates int
		for cur.Next() {
			ev := cur.Event()
			switch ev.Type {
			case marketdata.EventTypeBookSnapshot:
				snapshots++
			case marketdata.EventTypeBookUpdate:
				updates++
			}

			if err := book.Apply(ev); err != nil {
				t.Logf("book apply failed at %s: %v", ev.Time().Format(time.RFC3339Nano), err)
			}
		}
		require.NoError(t, cur.Err())

		t.Logf("%d snapshots, %d updates, %d unverified", snapshots, updates, book.Unverified())

		// Record what the answers turn out to be, so the package documentation
		// can be corrected from a real run.
		if snapshots > 0 {
			bid, ask, ok := book.Book.BestBidAndAsk()
			require.True(t, ok)
			t.Logf("reconstructed best bid/ask: %s / %s", bid.Price.String(), ask.Price.String())
			t.Logf("depth: %d bids, %d asks",
				len(book.Book.SideBook(types.SideTypeBuy)),
				len(book.Book.SideBook(types.SideTypeSell)))
		}
	})
}

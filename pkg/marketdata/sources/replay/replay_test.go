package replay

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/mdtest"
	"github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

const testExchange = types.ExchangeBinance

func testWriter(t *testing.T, dir string) *Writer {
	t.Helper()

	w, err := NewWriter(WriterConfig{
		Dir:          dir,
		Exchange:     testExchange,
		Symbols:      []string{"BTCUSDT"},
		Channels:     []types.Channel{types.BookChannel, types.MarketTradeChannel},
		Uncompressed: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { w.Close() })

	return w
}

func fullRange() marketdata.Request {
	return marketdata.Request{
		Since: time.Unix(0, 0),
		Until: time.Unix(0, 0).AddDate(100, 0, 0),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.BookChannel},
			{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel},
			{Symbol: "BTCUSDT", Channel: types.KLineChannel,
				Options: types.SubscribeOptions{Interval: types.Interval1m}},
			{Symbol: "BTCUSDT", Channel: types.BookTickerChannel},
		},
	}
}

// TestRoundTrip is the format's contract: everything written comes back with
// the fields a replay depends on intact.
func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	w := testWriter(t, dir)

	events := mdtest.Events(t, `
		t=1000 bookSnapshot seq=100 bids=100,10;99,5 asks=101,10;102,20
		t=1100 trade price=100.5 qty=0.25 seq=7001
		t=1200 bookUpdate   seq=101 bids=100,0
		t=60000 kline interval=1m o=100 h=102 l=99 c=101 v=12.5
		t=60100 bookTicker seq=102 bid=100.1,3 ask=100.2,4
	`)

	for i := range events {
		require.NoError(t, w.WriteEvent(&events[i]))
	}
	require.NoError(t, w.Close())

	src, err := New(Config{Path: dir})
	require.NoError(t, err)

	cur, err := src.Open(context.Background(), fullRange())
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, len(events))

	for i, want := range events {
		have := got[i]

		assert.Equal(t, want.Type, have.Type, "event %d type", i)
		assert.Equal(t, want.Key.TimeNs, have.Key.TimeNs, "event %d time", i)
		assert.Equal(t, want.Key.Rank, have.Key.Rank, "event %d rank", i)
		assert.Equal(t, want.Key.Seq, have.Key.Seq, "event %d seq", i)
		assert.Equal(t, "BTCUSDT", have.Symbol)
		assert.Equal(t, testExchange, have.Exchange)
	}

	// spot-check each payload kind survived
	require.NotNil(t, got[0].Book)
	assert.Equal(t, "100", got[0].Book.Bids[0].Price.String())
	assert.Equal(t, "10", got[0].Book.Bids[0].Volume.String())
	assert.Equal(t, int64(100), got[0].Book.LastUpdateId)

	require.NotNil(t, got[1].Trade)
	assert.Equal(t, "100.5", got[1].Trade.Price.String())
	assert.Equal(t, "0.25", got[1].Trade.Quantity.String())

	require.NotNil(t, got[2].Book)
	assert.Equal(t, "0", got[2].Book.Bids[0].Volume.String(),
		"a zero-volume level must survive the round trip: it means removal")

	require.NotNil(t, got[3].KLine)
	assert.Equal(t, types.Interval1m, got[3].KLine.Interval)
	assert.Equal(t, "101", got[3].KLine.Close.String())
	assert.Equal(t, int64(0), got[3].KLine.StartTime.Time().UnixMilli())

	require.NotNil(t, got[4].BookTicker)
	assert.Equal(t, "100.1", got[4].BookTicker.Buy.String())
}

func TestWriter_RotatesByHour(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWriter(WriterConfig{
		Dir: dir, Exchange: testExchange, Symbols: []string{"BTCUSDT"},
		Uncompressed: true, Rotate: time.Hour,
	})
	require.NoError(t, err)

	base := time.Date(2026, 9, 15, 10, 30, 0, 0, time.UTC)
	for _, offset := range []time.Duration{0, 20 * time.Minute, 40 * time.Minute} {
		ev := marketdata.Event{
			Type:     marketdata.EventTypeTrade,
			Exchange: testExchange,
			Symbol:   "BTCUSDT",
			Key: marketdata.OrderKey{
				TimeNs: base.Add(offset).UnixNano(),
				Rank:   marketdata.RankTrade,
			},
			Trade: &types.Trade{Symbol: "BTCUSDT", Price: testhelper.Number("100"), Quantity: testhelper.Number("1")},
		}
		require.NoError(t, w.WriteEvent(&ev))
	}
	require.NoError(t, w.Close())

	names, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	require.NoError(t, err)
	require.Len(t, names, 2, "10:30 and 10:50 share an hour, 11:10 starts a new file")

	assert.Contains(t, filepath.Base(names[0]), "20260915T10")
	assert.Contains(t, filepath.Base(names[1]), "20260915T11")
}

// TestSource_FiltersRangeSymbolAndChannel checks the cheap pre-decode filter.
func TestSource_FiltersRangeSymbolAndChannel(t *testing.T) {
	dir := t.TempDir()
	w := testWriter(t, dir)

	events := mdtest.Events(t, `
		t=1000 trade sym=BTCUSDT
		t=2000 trade sym=ETHUSDT
		t=3000 bookSnapshot sym=BTCUSDT seq=1 bids=100,1 asks=101,1
		t=9000 trade sym=BTCUSDT
	`)
	for i := range events {
		require.NoError(t, w.WriteEvent(&events[i]))
	}
	require.NoError(t, w.Close())

	src, err := New(Config{Path: dir, Symbols: []string{"BTCUSDT"}})
	require.NoError(t, err)

	req := marketdata.Request{
		Since: time.UnixMilli(1000),
		Until: time.UnixMilli(4000),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel},
		},
	}

	cur, err := src.Open(context.Background(), req)
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 1, "only the in-range BTCUSDT trade should survive")
	assert.Equal(t, int64(1000), got[0].Key.TimeNs/1e6)
}

func TestSource_RejectsUnknownFormatVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	require.NoError(t, os.WriteFile(path,
		[]byte(`{"v":99,"exchange":"binance","symbols":["BTCUSDT"],"startedAt":0}`+"\n"), 0o644))

	_, err := New(Config{Path: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "format version 99")
}

func TestSource_EmptyPath(t *testing.T) {
	_, err := New(Config{Path: t.TempDir()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no recordings found")
}

func TestSource_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	w := testWriter(t, dir)

	events := mdtest.Events(t, "t=1000 trade\nt=2000 trade")
	for i := range events {
		require.NoError(t, w.WriteEvent(&events[i]))
	}
	require.NoError(t, w.Close())

	src, err := New(Config{Path: dir})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cur, err := src.Open(ctx, fullRange())
	require.NoError(t, err)
	defer cur.Close()

	cancel()
	assert.False(t, cur.Next())
	assert.ErrorIs(t, cur.Err(), context.Canceled)
}

func TestWriter_Gzip(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWriter(WriterConfig{
		Dir: dir, Exchange: testExchange, Symbols: []string{"BTCUSDT"},
	})
	require.NoError(t, err)

	events := mdtest.Events(t, "t=1000 bookSnapshot seq=1 bids=100,10 asks=101,10")
	require.NoError(t, w.WriteEvent(&events[0]))
	require.NoError(t, w.Close())

	names, err := filepath.Glob(filepath.Join(dir, "*"+FileExtension))
	require.NoError(t, err)
	require.Len(t, names, 1)

	src, err := New(Config{Path: dir})
	require.NoError(t, err)

	cur, err := src.Open(context.Background(), fullRange())
	require.NoError(t, err)
	defer cur.Close()

	assert.Len(t, mdtest.Collect(t, cur), 1)
}

package amberdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/mdtest"
	"github.com/c9s/bbgo/pkg/marketdata/sources/amberdata/amberdataapi"
	"github.com/c9s/bbgo/pkg/types"
)

// fakeClient scripts pages so the Source and paginator can be exercised without
// credentials. It is hand-written rather than generated: the interface has four
// methods and the tests need to control page chains and inject typed API errors.
type fakeClient struct {
	tradePages [][]amberdataapi.Trade
	bookPages  [][]amberdataapi.Book
	snapPages  [][]amberdataapi.Book
	ohlcvPages [][]amberdataapi.OHLCV

	// errOnWindow returns an error while the requested window is at least this
	// long, so the shrinking path can be tested.
	errOnWindow time.Duration
	errToReturn error

	tradeCalls  atomic.Int64
	cursorCalls atomic.Int64
	windows     []time.Duration
}

func (c *fakeClient) page(next string, pages [][]amberdataapi.Trade) amberdataapi.Page[amberdataapi.Trade] {
	idx := 0
	if next != "" {
		fmt.Sscanf(next, "cursor://%d", &idx)
	}
	if idx >= len(pages) {
		return amberdataapi.Page[amberdataapi.Trade]{}
	}

	out := amberdataapi.Page[amberdataapi.Trade]{Data: pages[idx]}
	if idx+1 < len(pages) {
		out.Next = fmt.Sprintf("cursor://%d", idx+1)
	}
	return out
}

func (c *fakeClient) GetTrades(
	ctx context.Context, q amberdataapi.Query, next string,
) (amberdataapi.Page[amberdataapi.Trade], error) {
	if next != "" {
		c.cursorCalls.Add(1)
		return c.page(next, c.tradePages), nil
	}

	c.tradeCalls.Add(1)
	window := q.Until.Sub(q.Since)
	c.windows = append(c.windows, window)

	if c.errToReturn != nil && c.errOnWindow > 0 && window >= c.errOnWindow {
		return amberdataapi.Page[amberdataapi.Trade]{}, c.errToReturn
	}

	return c.page("", c.tradePages), nil
}

func (c *fakeClient) GetOrderBookEvents(
	ctx context.Context, q amberdataapi.Query, next string,
) (amberdataapi.Page[amberdataapi.Book], error) {
	return bookPage(next, c.bookPages), nil
}

func (c *fakeClient) GetOrderBookSnapshots(
	ctx context.Context, q amberdataapi.Query, next string,
) (amberdataapi.Page[amberdataapi.Book], error) {
	return bookPage(next, c.snapPages), nil
}

func (c *fakeClient) GetOHLCV(
	ctx context.Context, q amberdataapi.Query, next string,
) (amberdataapi.Page[amberdataapi.OHLCV], error) {
	if len(c.ohlcvPages) == 0 {
		return amberdataapi.Page[amberdataapi.OHLCV]{}, nil
	}
	return amberdataapi.Page[amberdataapi.OHLCV]{Data: c.ohlcvPages[0]}, nil
}

func bookPage(next string, pages [][]amberdataapi.Book) amberdataapi.Page[amberdataapi.Book] {
	idx := 0
	if next != "" {
		fmt.Sscanf(next, "cursor://%d", &idx)
	}
	if idx >= len(pages) {
		return amberdataapi.Page[amberdataapi.Book]{}
	}

	out := amberdataapi.Page[amberdataapi.Book]{Data: pages[idx]}
	if idx+1 < len(pages) {
		out.Next = fmt.Sprintf("cursor://%d", idx+1)
	}
	return out
}

func apiTrade(ms int64, id string, buy bool) amberdataapi.Trade {
	var t amberdataapi.Trade
	mustJSON(&t, fmt.Sprintf(
		`{"instrument":"BTCUSDT","exchange":"binance","exchangeTimestamp":%d,`+
			`"exchangeTimestampNanoseconds":0,"isBuySide":%t,"price":70845.5,"volume":21,`+
			`"tradeId":"%s","quoteVolume":null,"sequence":null}`, ms, buy, id))
	return t
}

func apiBook(ms int64, seq string, bids, asks string) amberdataapi.Book {
	var b amberdataapi.Book
	mustJSON(&b, fmt.Sprintf(
		`{"instrument":"BTCUSDT","exchange":"binance","exchangeTimestamp":%d,`+
			`"exchangeTimestampNanoseconds":0,"sequence":"%s","ask":[%s],"bid":[%s]}`,
		ms, seq, asks, bids))
	return b
}

func mustJSON(out any, raw string) {
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		panic(err)
	}
}

func baseRequest(subs ...types.Subscription) marketdata.Request {
	return marketdata.Request{
		Since:         time.UnixMilli(1_717_518_000_000),
		Until:         time.UnixMilli(1_717_518_000_000).Add(30 * time.Minute),
		Subscriptions: subs,
	}
}

func newSource(t *testing.T, client Client) *Source {
	t.Helper()

	src, err := New(Config{
		Exchange: "binance",
		Client:   client,
		Chunk:    time.Hour,
		MinChunk: time.Minute,
	})
	require.NoError(t, err)
	return src
}

func TestSource_Trades(t *testing.T) {
	client := &fakeClient{tradePages: [][]amberdataapi.Trade{
		{apiTrade(1_717_518_000_100, "1001", false), apiTrade(1_717_518_000_200, "1002", true)},
		{apiTrade(1_717_518_000_300, "1003", true)},
	}}

	src := newSource(t, client)

	cur, err := src.Open(context.Background(),
		baseRequest(types.Subscription{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel}))
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 3, "the cursor chain must be followed to exhaustion")

	assert.Equal(t, int64(1), client.cursorCalls.Load())

	first := got[0]
	assert.Equal(t, marketdata.EventTypeTrade, first.Type)
	assert.Equal(t, "70845.5", first.Trade.Price.String())
	assert.Equal(t, types.SideTypeSell, first.Trade.Side,
		"isBuySide reports the aggressor side directly, unlike binance's is_buyer_maker")
	assert.False(t, first.Trade.IsBuyer)
	assert.False(t, first.Trade.IsMaker, "a public trade has no maker side of its own")

	assert.Equal(t, uint64(1001), first.Key.Seq,
		"a null sequence must fall back to the numeric tradeId")
	assert.Equal(t, "1487755.5", first.Trade.QuoteQuantity.String(),
		"a null quoteVolume must be derived from price and volume")

	for i := 1; i < len(got); i++ {
		assert.LessOrEqual(t, got[i-1].Key.Compare(got[i].Key), 0)
	}
}

// TestSource_BookIsSnapshotsPlusEvents pins the two-stream mapping: the API has
// no endpoint returning both, and an events stream alone cannot initialize a book.
func TestSource_BookIsSnapshotsPlusEvents(t *testing.T) {
	client := &fakeClient{
		snapPages: [][]amberdataapi.Book{{
			apiBook(1_717_518_000_000, "5000",
				`{"price":70895.8,"volume":2097,"numOrders":null}`,
				`{"price":70895.9,"volume":7086,"numOrders":null}`),
		}},
		bookPages: [][]amberdataapi.Book{{
			apiBook(1_717_518_000_500, "5001",
				`{"price":70895.8,"volume":0,"numOrders":null}`, ``),
		}},
	}

	src := newSource(t, client)
	assert.Contains(t, src.Capabilities().Channels, types.BookChannel,
		"serving L2 is the reason this provider exists")

	cur, err := src.Open(context.Background(),
		baseRequest(types.Subscription{Symbol: "BTCUSDT", Channel: types.BookChannel}))
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 2)

	assert.Equal(t, marketdata.EventTypeBookSnapshot, got[0].Type,
		"the snapshots endpoint establishes the base state")
	assert.Equal(t, uint64(5000), got[0].Key.Seq)

	assert.Equal(t, marketdata.EventTypeBookUpdate, got[1].Type,
		"the events endpoint supplies the diffs")
	require.Len(t, got[1].Book.Bids, 1)
	assert.Equal(t, "0", got[1].Book.Bids[0].Volume.String(),
		"a zero volume must reach the consumer as-is: it means removal")

	// The two streams must apply cleanly to a book, which is the whole point.
	book := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
	for _, ev := range got {
		require.NoError(t, book.Apply(ev))
	}

	bid, _ := book.Book.BestBid()
	assert.True(t, bid.Price.IsZero(), "the only bid level was removed by the diff")
}

// TestSource_BookEventsCarryNoPrevSeq documents a deliberate gap: the API's
// sequence is not documented against binance's U/u/pu protocol, so claiming an
// update follows a specific predecessor would be a guess.
func TestSource_BookEventsCarryNoPrevSeq(t *testing.T) {
	client := &fakeClient{
		snapPages: [][]amberdataapi.Book{{
			apiBook(1_717_518_000_000, "5000",
				`{"price":100,"volume":1,"numOrders":null}`,
				`{"price":101,"volume":1,"numOrders":null}`),
		}},
		bookPages: [][]amberdataapi.Book{{
			apiBook(1_717_518_000_500, "5010", `{"price":100,"volume":2,"numOrders":null}`, ``),
		}},
	}

	src := newSource(t, client)
	cur, err := src.Open(context.Background(),
		baseRequest(types.Subscription{Symbol: "BTCUSDT", Channel: types.BookChannel}))
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 2)
	assert.Zero(t, got[1].PrevSeq)

	book := marketdata.NewBookState("BTCUSDT", types.ExchangeBinance)
	book.Mode = marketdata.SequenceContiguous
	for _, ev := range got {
		require.NoError(t, book.Apply(ev))
	}

	assert.Equal(t, int64(1), book.Unverified(),
		"without a previous-update id the update is counted as unverified, not rejected")
}

// TestSource_ShrinksWindowOnTooLarge covers the response to the 10 MB cap: halve
// the window, do not retry the same request.
func TestSource_ShrinksWindowOnTooLarge(t *testing.T) {
	tooLarge := amberdataapi.ParseAPIError(http.StatusBadRequest,
		[]byte(`{"status":400,"title":"Bad Request","description":"payload size exceeds 10 MB"}`))

	client := &fakeClient{
		tradePages:  [][]amberdataapi.Trade{{apiTrade(1_717_518_000_100, "1", true)}},
		errOnWindow: 20 * time.Minute,
		errToReturn: tooLarge,
	}

	src := newSource(t, client)

	cur, err := src.Open(context.Background(),
		baseRequest(types.Subscription{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel}))
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	assert.NotEmpty(t, got)

	require.GreaterOrEqual(t, len(client.windows), 2)
	assert.Greater(t, client.windows[0], client.windows[1],
		"the window must shrink rather than the request being retried unchanged")
}

func TestSource_GivesUpAtMinChunk(t *testing.T) {
	tooLarge := amberdataapi.ParseAPIError(http.StatusBadRequest,
		[]byte(`{"description":"payload size exceeds 10 MB"}`))

	client := &fakeClient{
		errOnWindow: time.Nanosecond, // always too large
		errToReturn: tooLarge,
	}

	src, err := New(Config{
		Exchange: "binance", Client: client,
		Chunk: 4 * time.Minute, MinChunk: time.Minute,
	})
	require.NoError(t, err)

	cur, err := src.Open(context.Background(),
		baseRequest(types.Subscription{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel}))
	require.NoError(t, err)
	defer cur.Close()

	for cur.Next() {
	}
	require.Error(t, cur.Err())
	assert.Contains(t, cur.Err().Error(), "minimum chunk",
		"shrinking must stop at the floor and report, not loop")
}

func TestSource_TimeoutAlsoShrinks(t *testing.T) {
	timeout := amberdataapi.ParseAPIError(http.StatusGatewayTimeout, []byte(`{}`))

	client := &fakeClient{
		tradePages:  [][]amberdataapi.Trade{{apiTrade(1_717_518_000_100, "1", true)}},
		errOnWindow: 20 * time.Minute,
		errToReturn: timeout,
	}

	src := newSource(t, client)
	cur, err := src.Open(context.Background(),
		baseRequest(types.Subscription{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel}))
	require.NoError(t, err)
	defer cur.Close()

	assert.NotEmpty(t, mdtest.Collect(t, cur))
	require.GreaterOrEqual(t, len(client.windows), 2)
	assert.Greater(t, client.windows[0], client.windows[1])
}

// TestSource_OtherErrorsPropagate checks that a 403 is not mistaken for a
// window problem and silently retried into oblivion.
func TestSource_OtherErrorsPropagate(t *testing.T) {
	forbidden := amberdataapi.ParseAPIError(http.StatusForbidden, []byte(`{"message":"Forbidden"}`))

	client := &fakeClient{errOnWindow: time.Nanosecond, errToReturn: forbidden}

	src := newSource(t, client)
	cur, err := src.Open(context.Background(),
		baseRequest(types.Subscription{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel}))
	require.NoError(t, err)
	defer cur.Close()

	for cur.Next() {
	}
	require.Error(t, cur.Err())
	assert.Contains(t, cur.Err().Error(), "not included in the plan")
	assert.Equal(t, int64(1), client.tradeCalls.Load(), "a 403 must not be retried")
}

func TestSource_RejectsUnsupportedInterval(t *testing.T) {
	src := newSource(t, &fakeClient{})

	_, err := src.Open(context.Background(), baseRequest(types.Subscription{
		Symbol: "BTCUSDT", Channel: types.KLineChannel,
		Options: types.SubscribeOptions{Interval: types.Interval1s},
	}))

	require.Error(t, err)
	var unsupported *marketdata.UnsupportedError
	require.ErrorAs(t, err, &unsupported)
	assert.Contains(t, unsupported.Reason, "minute, hour and day")
}

func TestSource_OHLCV(t *testing.T) {
	var candle amberdataapi.OHLCV
	mustJSON(&candle, `{"instrument":"BTCUSDT","exchange":"binance",`+
		`"exchangeTimestamp":1717518000000,"open":0.023,"high":0.026,"low":0.022392,`+
		`"close":0.02531,"volume":210710456}`)

	src := newSource(t, &fakeClient{ohlcvPages: [][]amberdataapi.OHLCV{{candle}}})

	cur, err := src.Open(context.Background(), baseRequest(types.Subscription{
		Symbol: "BTCUSDT", Channel: types.KLineChannel,
		Options: types.SubscribeOptions{Interval: types.Interval1h},
	}))
	require.NoError(t, err)
	defer cur.Close()

	got := mdtest.Collect(t, cur)
	require.Len(t, got, 1)

	kline := got[0].KLine
	require.NotNil(t, kline)
	assert.Equal(t, types.Interval1h, kline.Interval)
	assert.Equal(t, int64(1717518000000), kline.StartTime.Time().UnixMilli())
	assert.Equal(t, int64(1717518000000)+3600_000-1, kline.EndTime.Time().UnixMilli(),
		"the API reports the bucket start, but a kline event happens at its close")
	assert.Equal(t, kline.EndTime.Time().UnixNano(), got[0].Key.TimeNs)
}

func TestSource_ContextCancellation(t *testing.T) {
	client := &fakeClient{tradePages: [][]amberdataapi.Trade{
		{apiTrade(1_717_518_000_100, "1", true)},
	}}

	src := newSource(t, client)

	ctx, cancel := context.WithCancel(context.Background())
	cur, err := src.Open(ctx, baseRequest(
		types.Subscription{Symbol: "BTCUSDT", Channel: types.MarketTradeChannel}))
	require.NoError(t, err)
	defer cur.Close()

	cancel()
	for cur.Next() {
	}
	// Cancellation surfaces as the context error, matching every other source in
	// the layer, so a caller can tell a cancelled run from an exhausted one.
	assert.ErrorIs(t, cur.Err(), context.Canceled)
}

func TestConfig_Validation(t *testing.T) {
	t.Run("exchange required", func(t *testing.T) {
		_, err := New(Config{APIKey: "UATx"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exchange is required")
	})

	t.Run("key required without a client", func(t *testing.T) {
		_, err := New(Config{Exchange: "binance"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "apiKey is required")
	})

	t.Run("unknown asset class", func(t *testing.T) {
		_, err := New(Config{Exchange: "binance", APIKey: "UATx", AssetClass: "options"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown asset class")
	})

	// The tier's documented rate is halved, so a backfill leaves room for
	// whatever else shares the key.
	t.Run("rate derived from the key tier", func(t *testing.T) {
		cfg := Config{Exchange: "binance", APIKey: "UATtestkey"}
		cfg.applyDefaults()
		assert.Equal(t, 7.5, cfg.RequestsPerSecond)
	})

	t.Run("rate for an unknown prefix is conservative", func(t *testing.T) {
		cfg := Config{Exchange: "binance", APIKey: "ZZZtestkey"}
		cfg.applyDefaults()
		assert.Equal(t, float64(5), cfg.RequestsPerSecond)
	})
}

func TestOHLCVInterval(t *testing.T) {
	for _, tt := range []struct {
		interval types.Interval
		want     string
		ok       bool
	}{
		{types.Interval1m, "minutes", true},
		{types.Interval30m, "minutes", true},
		{types.Interval1h, "hours", true},
		{types.Interval4h, "hours", true},
		{types.Interval1d, "days", true},
		{types.Interval1s, "", false},
		{types.Interval1w, "", false},
	} {
		got, ok := ohlcvInterval(tt.interval.Duration())
		assert.Equal(t, tt.ok, ok, tt.interval)
		assert.Equal(t, tt.want, got, tt.interval)
	}
}

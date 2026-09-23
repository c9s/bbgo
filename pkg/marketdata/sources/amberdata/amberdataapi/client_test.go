package amberdataapi

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/testing/httptesting"
)

// newTestClient wires a RestClient to a MockTransport serving fixtures, and
// captures the last request so the headers and query can be asserted.
func newTestClient(t *testing.T, register func(*httptesting.MockTransport)) (*RestClient, **http.Request) {
	t.Helper()

	transport := &httptesting.MockTransport{}
	register(transport)

	var captured *http.Request
	client := NewRestClient()
	client.Auth("UATtestkey000")
	client.HttpClient = &http.Client{Transport: capturingTransport{
		inner:    transport,
		captured: &captured,
	}}

	return client, &captured
}

type capturingTransport struct {
	inner    http.RoundTripper
	captured **http.Request
}

func (t capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	*t.captured = req.Clone(req.Context())
	return t.inner.RoundTrip(req)
}

func serveFixture(t *testing.T, transport *httptesting.MockTransport, path, fixture string) {
	t.Helper()

	body, err := os.ReadFile("testdata/" + fixture)
	require.NoError(t, err)

	transport.GET(path, func(req *http.Request) (*http.Response, error) {
		return httptesting.BuildResponse(http.StatusOK, body), nil
	})
}

// TestGetTrades_SendsRequiredHeadersAndParams pins the request shape. The two
// headers are documented as required, and timeFormat has to be sent because the
// server's default emits a non-numeric, non-RFC3339 timestamp.
func TestGetTrades_SendsRequiredHeadersAndParams(t *testing.T) {
	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport, "/markets/futures/trades/BTCUSD_PERP", "futures_trades.json")
	})

	since := time.UnixMilli(1717518000000)
	until := time.UnixMilli(1717519000000)

	_, err := client.GetTrades(context.Background(), AssetClassFutures, Query{
		Exchange:   "binance",
		Instrument: "BTCUSD_PERP",
		Since:      since,
		Until:      until,
	})
	require.NoError(t, err)

	req := *captured
	require.NotNil(t, req)

	assert.Equal(t, "UATtestkey000", req.Header.Get("x-api-key"))
	assert.Equal(t, "gzip, deflate, br", req.Header.Get("Accept-Encoding"))
	assert.Equal(t, DefaultAPIVersion, req.Header.Get("api-version"))

	q := req.URL.Query()
	assert.Equal(t, "binance", q.Get("exchange"))
	assert.Equal(t, "milliseconds", q.Get("timeFormat"),
		"the default of hr emits a space-separated millisecond field, so this must always be sent")
	assert.Equal(t, "1717518000000", q.Get("startDate"))
	assert.Equal(t, "1717519000000", q.Get("endDate"))
	assert.Empty(t, q.Get("sortDirection"),
		"sortDirection is rejected on a historical range, so it must never be sent")
}

func TestGetTrades_DecodesFixture(t *testing.T) {
	client, _ := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport, "/markets/futures/trades/BTCUSD_PERP", "futures_trades.json")
	})

	page, err := client.GetTrades(context.Background(), AssetClassFutures, Query{
		Exchange: "binance", Instrument: "BTCUSD_PERP",
	})
	require.NoError(t, err)

	require.Len(t, page.Data, 2)
	assert.Contains(t, page.Next, "cursor=", "the cursor URL must be preserved verbatim")

	first := page.Data[0]
	assert.Equal(t, "70845.5", first.Price.String())
	assert.Equal(t, "21", first.Volume.String())
	assert.Equal(t, "819875589", first.TradeID, "tradeId is a string in this API")
	assert.False(t, first.IsBuySide)

	assert.False(t, first.QuoteVolume.Valid, "a null quoteVolume must decode as absent, not zero")
	assert.False(t, first.Sequence.Valid, "a null sequence must decode as absent")

	assert.Equal(t, int64(1717518651671), first.ExchangeTimestamp.UnixMilli())
	assert.Equal(t, int64(1717518651671)*1e6, first.EventTimeNano())
}

// TestGetOrderBookEvents_DecodesFixture covers the two things the events payload
// gets right that a hand-written fixture would have missed: sequence arrives as a
// quoted string here even though trades type it as a number, and a zero volume is
// a removal rather than a missing value.
func TestGetOrderBookEvents_DecodesFixture(t *testing.T) {
	client, _ := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport,
			"/markets/futures/order-book-events/BTCUSD_PERP", "futures_order_book_events.json")
	})

	page, err := client.GetOrderBookEvents(context.Background(), AssetClassFutures, Query{
		Exchange: "binance", Instrument: "BTCUSD_PERP",
	})
	require.NoError(t, err)

	require.Len(t, page.Data, 1)
	book := page.Data[0]

	require.True(t, book.Sequence.Valid)
	assert.Equal(t, uint64(960251880359), book.Sequence.Value,
		"a quoted sequence must decode as a number")

	require.Len(t, book.Ask, 5)
	assert.Equal(t, "70976", book.Ask[3].Price.String())
	assert.Equal(t, "0", book.Ask[3].Volume.String(),
		"a zero volume is the removal signal and must survive decoding")

	assert.Empty(t, page.Next, "a null cursor means the result set is exhausted")
	assert.Equal(t, int64(1717518192414), page.ReturnedEnd.UnixMilli())
}

// TestGetOrderBookEvents_HumanReadableTimestamps decodes the same payload as the
// API returns it without timeFormat=milliseconds. The client always sends the
// parameter, but a fixture transcribed from the docs uses this encoding, and
// tolerating it means a lost parameter degrades rather than fails.
func TestGetOrderBookEvents_HumanReadableTimestamps(t *testing.T) {
	client, _ := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport,
			"/markets/futures/order-book-events/BTCUSD_PERP", "futures_order_book_events_hr.json")
	})

	page, err := client.GetOrderBookEvents(context.Background(), AssetClassFutures, Query{
		Exchange: "binance", Instrument: "BTCUSD_PERP",
	})
	require.NoError(t, err)

	require.Len(t, page.Data, 1)
	assert.Equal(t, "2024-06-04T16:23:12.414Z",
		page.Data[0].ExchangeTimestamp.Format("2006-01-02T15:04:05.000Z"))
}

func TestGetOrderBookSnapshots_DecodesFixture(t *testing.T) {
	client, _ := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport,
			"/markets/futures/order-book-snapshots/BTCUSD_PERP", "futures_order_book_snapshots.json")
	})

	page, err := client.GetOrderBookSnapshots(context.Background(), AssetClassFutures, Query{
		Exchange: "binance", Instrument: "BTCUSD_PERP", MaxLevel: 20,
	})
	require.NoError(t, err)

	require.Len(t, page.Data, 1)
	book := page.Data[0]

	assert.Equal(t, uint64(960253327911), book.Sequence.Value)
	assert.False(t, book.CurrentFunding.Valid, "a null currentFunding must decode as absent")

	// The exchange timestamp is preferred over the ingestion timestamp: only the
	// former is meaningful under simulated time.
	assert.Equal(t, int64(1717518300711), book.ExchangeTimestamp.UnixMilli())
	assert.Equal(t, int64(1717518300000), book.Timestamp.UnixMilli())
	assert.Equal(t, int64(1717518300711)*1e6, book.EventTimeNano())
}

func TestGetOrderBookSnapshots_SendsMaxLevel(t *testing.T) {
	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport,
			"/markets/futures/order-book-snapshots/BTCUSD_PERP", "futures_order_book_snapshots.json")
	})

	_, err := client.GetOrderBookSnapshots(context.Background(), AssetClassFutures, Query{
		Exchange: "binance", Instrument: "BTCUSD_PERP", MaxLevel: 20,
	})
	require.NoError(t, err)

	assert.Equal(t, "20", (*captured).URL.Query().Get("maxLevel"))
}

func TestGetOHLCV_DecodesFixture(t *testing.T) {
	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport, "/markets/futures/ohlcv/1000BONKUSDC", "futures_ohlcv.json")
	})

	page, err := client.GetOHLCV(context.Background(), AssetClassFutures, Query{
		Exchange: "binance", Instrument: "1000BONKUSDC", TimeInterval: "hours",
	})
	require.NoError(t, err)

	assert.Equal(t, "hours", (*captured).URL.Query().Get("timeInterval"))

	require.Len(t, page.Data, 1)
	assert.Equal(t, "0.023", page.Data[0].Open.String())
	assert.Equal(t, "0.02531", page.Data[0].Close.String())
	assert.Equal(t, int64(1714608000000), page.Data[0].ExchangeTimestamp.UnixMilli())
}

func TestQuery_RequiresExchange(t *testing.T) {
	client, _ := newTestClient(t, func(transport *httptesting.MockTransport) {})

	_, err := client.GetTrades(context.Background(), AssetClassFutures, Query{
		Instrument: "BTCUSDT",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exchange is required")
}

func TestGetCursor_ReissuesURLVerbatim(t *testing.T) {
	const cursorPath = "/markets/futures/trades/BTCUSD_PERP"

	client, captured := newTestClient(t, func(transport *httptesting.MockTransport) {
		serveFixture(t, transport, cursorPath, "futures_trades.json")
	})

	cursorURL := "https://api.amberdata.com" + cursorPath + "?cursor=OPAQUE123"

	_, err := GetCursor[Trade](context.Background(), client, cursorURL)
	require.NoError(t, err)

	req := *captured
	assert.Equal(t, "OPAQUE123", req.URL.Query().Get("cursor"),
		"the opaque cursor must be re-issued exactly, not reconstructed")
	assert.Equal(t, "UATtestkey000", req.Header.Get("x-api-key"),
		"a cursor request still needs the auth headers")
	assert.Equal(t, "gzip, deflate, br", req.Header.Get("Accept-Encoding"))
}

func TestTierFromKey(t *testing.T) {
	for key, want := range map[string]string{
		"UATabc": "trial",
		"UAOabc": "on-demand",
		"UAKabc": "enterprise",
	} {
		tier, ok := TierFromKey(key)
		require.True(t, ok, key)
		assert.Equal(t, want, tier.Name)
	}

	_, ok := TierFromKey("XYZabc")
	assert.False(t, ok, "an unknown prefix must not be guessed at")

	_, ok = TierFromKey("")
	assert.False(t, ok)
}

func TestNormalizeInstrument(t *testing.T) {
	assert.Equal(t, "BTCUSDT", NormalizeInstrument(AssetClassFutures, "BTCUSDT"))
	assert.Equal(t, "BTCUSD_PERP", NormalizeInstrument(AssetClassFutures, "btcusd_perp"))
	assert.Equal(t, "btc_usd", NormalizeInstrument(AssetClassSpot, "BTC_USD"))
}

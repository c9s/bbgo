package binancecsv

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/archive"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/types"
)

// fixtureServer serves fixture-backed archives from a MockTransport, zipping the
// fixture on the fly so the test exercises the real streaming path.
type fixtureServer struct {
	transport *httptesting.MockTransport
	requests  atomic.Int64
	missing   map[string]bool
}

func newFixtureServer() *fixtureServer {
	return &fixtureServer{
		transport: &httptesting.MockTransport{},
		missing:   map[string]bool{},
	}
}

// serve registers ref, answering with a zip built from the given fixture file.
func (s *fixtureServer) serve(t *testing.T, ref FileRef, fixture string) {
	t.Helper()

	content, err := os.ReadFile(fixture)
	require.NoError(t, err)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(path.Base(fixture))
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	payload := buf.Bytes()
	urlPath := "/" + ref.URLPath()

	s.transport.GET(urlPath, func(req *http.Request) (*http.Response, error) {
		s.requests.Add(1)
		if s.missing[urlPath] {
			return httptesting.BuildResponse(http.StatusNotFound, nil), nil
		}
		return httptesting.BuildResponse(http.StatusOK, payload), nil
	})

	// no sidecar in these tests
	s.transport.GET(urlPath+".CHECKSUM", func(req *http.Request) (*http.Response, error) {
		return httptesting.BuildResponse(http.StatusNotFound, nil), nil
	})
}

// serveShifted registers ref, answering with the fixture's records moved
// forward by dayOffset days. Real archives never repeat timestamps across days,
// so a multi-day test has to shift them to stay faithful.
func (s *fixtureServer) serveShifted(t *testing.T, ref FileRef, fixture string, dayOffset int, timeCols ...int) {
	t.Helper()

	raw, err := os.ReadFile(fixture)
	require.NoError(t, err)

	shift := int64(dayOffset) * 24 * 60 * 60 * 1000 // fixture timestamps are in ms

	var out bytes.Buffer
	for i, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		fields := strings.Split(line, ",")
		if i == 0 && !isNumeric(fields[0]) {
			out.WriteString(line + "\n")
			continue
		}

		for _, col := range timeCols {
			ms, err := strconv.ParseInt(fields[col], 10, 64)
			require.NoError(t, err, "column %d of %q", col, line)
			fields[col] = strconv.FormatInt(ms+shift, 10)
		}

		out.WriteString(strings.Join(fields, ",") + "\n")
	}

	path := t.TempDir() + "/shifted.csv"
	require.NoError(t, os.WriteFile(path, out.Bytes(), 0o644))

	s.serve(t, ref, path)
}

func isNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// absent registers ref as a 404, the way the publisher answers for a day it
// never produced.
func (s *fixtureServer) absent(t *testing.T, ref FileRef) {
	t.Helper()

	urlPath := "/" + ref.URLPath()
	s.missing[urlPath] = true

	s.transport.GET(urlPath, func(req *http.Request) (*http.Response, error) {
		s.requests.Add(1)
		return httptesting.BuildResponse(http.StatusNotFound, nil), nil
	})
	s.transport.GET(urlPath+".CHECKSUM", func(req *http.Request) (*http.Response, error) {
		return httptesting.BuildResponse(http.StatusNotFound, nil), nil
	})
}

func (s *fixtureServer) cache(dir string) *archive.HTTPCache {
	return &archive.HTTPCache{
		Dir:        dir,
		HTTPClient: &http.Client{Transport: s.transport},
		Limiter:    rate.NewLimiter(rate.Inf, 1),
	}
}

func aggTradeRef(t *testing.T, day string) FileRef {
	return FileRef{
		Market:  MarketUSDMFutures,
		Period:  PeriodDaily,
		Dataset: DatasetAggTrades,
		Symbol:  "BTCUSDT",
		Date:    date(t, day),
	}
}

func tradeRequest(t *testing.T, since, until string) marketdata.Request {
	return marketdata.Request{
		Since: date(t, since),
		Until: date(t, until),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.AggTradeChannel},
		},
	}
}

// TestSource_StreamsAcrossFileBoundaries is the core cursor test: three
// consecutive daily archives must come out as one non-decreasing stream.
func TestSource_StreamsAcrossFileBoundaries(t *testing.T) {
	srv := newFixtureServer()
	for offset, day := range []string{"2026-09-15", "2026-09-16", "2026-09-17"} {
		srv.serveShifted(t, aggTradeRef(t, day),
			"testdata/um/BTCUSDT-aggTrades-2026-09-15.csv", offset, 5)
	}

	src, err := New(Config{
		Market:   MarketUSDMFutures,
		Datasets: []string{DatasetAggTrades},
		Cache:    srv.cache(t.TempDir()),
	})
	require.NoError(t, err)

	cur, err := src.Open(context.Background(), tradeRequest(t, "2026-09-15", "2026-09-18"))
	require.NoError(t, err)
	defer cur.Close()

	var events []marketdata.Event
	for cur.Next() {
		events = append(events, *cur.Event().Clone())
	}
	require.NoError(t, cur.Err())

	require.NotEmpty(t, events)
	for i := 1; i < len(events); i++ {
		assert.LessOrEqual(t, events[i-1].Key.Compare(events[i].Key), 0,
			"the stream must stay ordered across archive boundaries at index %d", i)
	}
	assert.Equal(t, int64(3), srv.requests.Load(),
		"each archive must be fetched exactly once")
}

// TestSource_FiltersEventsOutsideRange covers the half-open range: a daily
// archive spans the whole UTC day, so a request that starts mid-day must not
// leak the earlier events into the merge.
func TestSource_FiltersEventsOutsideRange(t *testing.T) {
	srv := newFixtureServer()
	srv.serve(t, aggTradeRef(t, "2026-09-15"), "testdata/um/BTCUSDT-aggTrades-2026-09-15.csv")

	src, err := New(Config{
		Market:   MarketUSDMFutures,
		Datasets: []string{DatasetAggTrades},
		Cache:    srv.cache(t.TempDir()),
	})
	require.NoError(t, err)

	// The fixture's five records all sit within the first 20ms of the day.
	req := marketdata.Request{
		Since: date(t, "2026-09-15").Add(time.Hour),
		Until: date(t, "2026-09-16"),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.AggTradeChannel},
		},
	}

	cur, err := src.Open(context.Background(), req)
	require.NoError(t, err)
	defer cur.Close()

	var count int
	for cur.Next() {
		count++
	}
	require.NoError(t, cur.Err())
	assert.Zero(t, count, "events before Since must be dropped")
}

// TestSource_MissingDay is the regression test for the bug in the code this
// replaces: csvsource.Download logged the error, broke out of its loop and
// returned nil, so a 404 mid-range produced a short dataset that looked
// complete.
func TestSource_MissingDay(t *testing.T) {
	newSource := func(t *testing.T, allowMissing bool) marketdata.Source {
		srv := newFixtureServer()
		srv.serveShifted(t, aggTradeRef(t, "2026-09-15"),
			"testdata/um/BTCUSDT-aggTrades-2026-09-15.csv", 0, 5)
		srv.absent(t, aggTradeRef(t, "2026-09-16"))
		srv.serveShifted(t, aggTradeRef(t, "2026-09-17"),
			"testdata/um/BTCUSDT-aggTrades-2026-09-15.csv", 2, 5)

		src, err := New(Config{
			Market:           MarketUSDMFutures,
			Datasets:         []string{DatasetAggTrades},
			Cache:            srv.cache(t.TempDir()),
			AllowMissingDays: allowMissing,
		})
		require.NoError(t, err)
		return src
	}

	t.Run("fails by default", func(t *testing.T) {
		src := newSource(t, false)

		cur, err := src.Open(context.Background(), tradeRequest(t, "2026-09-15", "2026-09-18"))
		require.NoError(t, err)
		defer cur.Close()

		for cur.Next() {
		}

		require.Error(t, cur.Err(), "a gap must not be mistaken for the end of the data")
		var missing *archive.MissingArchiveError
		assert.ErrorAs(t, cur.Err(), &missing)
	})

	t.Run("tolerated when opted in", func(t *testing.T) {
		src := newSource(t, true)

		cur, err := src.Open(context.Background(), tradeRequest(t, "2026-09-15", "2026-09-18"))
		require.NoError(t, err)
		defer cur.Close()

		var count int
		for cur.Next() {
			count++
		}

		require.NoError(t, cur.Err())
		assert.Equal(t, 10, count, "the two available days must still be served")
	})
}

// TestSource_RejectsBookChannel is the honest-failure test: this source can
// never serve L2, so it must say so rather than return an empty cursor.
func TestSource_RejectsBookChannel(t *testing.T) {
	src, err := New(Config{
		Market:   MarketUSDMFutures,
		Datasets: []string{DatasetAggTrades, DatasetBookTicker},
		CacheDir: t.TempDir(),
	})
	require.NoError(t, err)

	assert.NotContains(t, src.Capabilities().Channels, types.BookChannel)

	req := marketdata.Request{
		Since: date(t, "2026-09-15"),
		Until: date(t, "2026-09-16"),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.BookChannel},
		},
	}

	_, err = src.Open(context.Background(), req)
	require.Error(t, err)

	var unsupported *marketdata.UnsupportedError
	require.ErrorAs(t, err, &unsupported)
	assert.Contains(t, unsupported.Reason, "publishes no L2 order book data")
}

// TestSource_BookTickerCoverage checks that the discontinued dataset fails with
// its real publication window rather than with a wall of 404s.
func TestSource_BookTickerCoverage(t *testing.T) {
	src, err := New(Config{
		Market:   MarketUSDMFutures,
		Datasets: []string{DatasetBookTicker},
		CacheDir: t.TempDir(),
	})
	require.NoError(t, err)

	req := marketdata.Request{
		Since: date(t, "2026-09-15"),
		Until: date(t, "2026-09-16"),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.BookTickerChannel},
		},
	}

	_, err = src.Open(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discontinued after 2024-03-31")
}

func TestSource_ContextCancellation(t *testing.T) {
	srv := newFixtureServer()
	srv.serve(t, aggTradeRef(t, "2026-09-15"), "testdata/um/BTCUSDT-aggTrades-2026-09-15.csv")

	src, err := New(Config{
		Market:   MarketUSDMFutures,
		Datasets: []string{DatasetAggTrades},
		Cache:    srv.cache(t.TempDir()),
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cur, err := src.Open(ctx, tradeRequest(t, "2026-09-15", "2026-09-16"))
	require.NoError(t, err)
	defer cur.Close()

	cancel()

	assert.False(t, cur.Next())
	assert.ErrorIs(t, cur.Err(), context.Canceled)
}

func TestSource_ConfigValidation(t *testing.T) {
	t.Run("unknown dataset", func(t *testing.T) {
		_, err := New(Config{Market: MarketSpot, Datasets: []string{"depthDiff"}, CacheDir: "x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown dataset")
	})

	t.Run("dataset not published for market", func(t *testing.T) {
		_, err := New(Config{Market: MarketSpot, Datasets: []string{DatasetBookTicker}, CacheDir: "x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not published for market spot")
	})

	t.Run("no dataset", func(t *testing.T) {
		_, err := New(Config{Market: MarketSpot, CacheDir: "x"})
		assert.Error(t, err)
	})

	t.Run("no cache", func(t *testing.T) {
		_, err := New(Config{Market: MarketSpot, Datasets: []string{DatasetAggTrades}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cacheDir")
	})

	t.Run("kline dataset without an interval", func(t *testing.T) {
		src, err := New(Config{
			Market: MarketSpot, Datasets: []string{DatasetKLines}, CacheDir: t.TempDir(),
		})
		require.NoError(t, err)

		req := marketdata.Request{
			Since: date(t, "2026-09-15"),
			Until: date(t, "2026-09-16"),
			Subscriptions: []types.Subscription{
				{Symbol: "BTCUSDT", Channel: types.KLineChannel},
			},
		}

		_, err = src.Open(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "needs an interval")
	})
}

func TestSource_Files(t *testing.T) {
	src, err := New(Config{
		Market:    MarketUSDMFutures,
		Datasets:  []string{DatasetKLines},
		Intervals: []types.Interval{types.Interval1h},
		CacheDir:  t.TempDir(),
	})
	require.NoError(t, err)

	req := marketdata.Request{
		Since: date(t, "2026-09-15"),
		Until: date(t, "2026-09-17"),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.KLineChannel,
				Options: types.SubscribeOptions{Interval: types.Interval1h}},
		},
	}

	refs, err := src.Files(req)
	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, "BTCUSDT-1h-2026-09-15.zip", refs[0].FileName())
}

// TestSource_MergesWithAnotherSource is the point of the whole layer: two
// datasets from the same tree, interleaved by time.
func TestSource_MergesWithAnotherSource(t *testing.T) {
	srv := newFixtureServer()
	srv.serve(t, aggTradeRef(t, "2026-09-15"), "testdata/um/BTCUSDT-aggTrades-2026-09-15.csv")

	klineRef := FileRef{
		Market: MarketUSDMFutures, Period: PeriodDaily, Dataset: DatasetKLines,
		Symbol: "BTCUSDT", Interval: types.Interval1h, Date: date(t, "2026-09-15"),
	}
	srv.serve(t, klineRef, "testdata/um/BTCUSDT-1h-2026-09-15.csv")

	cache := srv.cache(t.TempDir())

	trades, err := New(Config{
		Name: "trades", Market: MarketUSDMFutures,
		Datasets: []string{DatasetAggTrades}, Cache: cache,
	})
	require.NoError(t, err)

	klines, err := New(Config{
		Name: "klines", Market: MarketUSDMFutures,
		Datasets: []string{DatasetKLines}, Intervals: []types.Interval{types.Interval1h},
		Cache: cache,
	})
	require.NoError(t, err)

	req := marketdata.Request{
		Since: date(t, "2026-09-15"),
		Until: date(t, "2026-09-16"),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.AggTradeChannel},
			{Symbol: "BTCUSDT", Channel: types.KLineChannel,
				Options: types.SubscribeOptions{Interval: types.Interval1h}},
		},
	}

	m, err := marketdata.MergeSources(context.Background(),
		[]marketdata.Source{trades, klines}, req)
	require.NoError(t, err)
	defer m.Close()

	var sawTrade, sawKLine bool
	var prev marketdata.OrderKey
	for m.Next() {
		ev := m.Event()
		switch ev.Type {
		case marketdata.EventTypeTrade:
			sawTrade = true
		case marketdata.EventTypeKLine:
			sawKLine = true
		}
		assert.LessOrEqual(t, prev.Compare(ev.Key), 0)
		prev = ev.Key
	}
	require.NoError(t, m.Err())

	assert.True(t, sawTrade, "trades must be present")
	assert.True(t, sawKLine, "klines must be present")
}

func TestReadKLineFile(t *testing.T) {
	klines, err := ReadKLineFile(
		"testdata/um/BTCUSDT-1h-2026-09-15.csv", "BTCUSDT", types.Interval1h)
	require.NoError(t, err)

	require.Len(t, klines, 3)
	assert.Equal(t, "BTCUSDT", klines[0].Symbol)
	assert.Equal(t, types.Interval1h, klines[0].Interval)
	assert.Equal(t, "78153", klines[0].Open.String())
}

// TestReadKLineDir is the parity test for the xhedgegrid migration: the
// replacement for csvsource.ReadAllKLineCsv must produce the same candles.
func TestReadKLineDir(t *testing.T) {
	dir := t.TempDir()

	content, err := os.ReadFile("testdata/um/BTCUSDT-1h-2026-09-15.csv")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dir+"/a.csv", content, 0o644))
	require.NoError(t, os.WriteFile(dir+"/ignored.txt", []byte("nope"), 0o644))

	klines, err := ReadKLineDir(dir, "BTCUSDT", types.Interval1h)
	require.NoError(t, err)

	require.Len(t, klines, 3)
	for i := 1; i < len(klines); i++ {
		assert.True(t, klines[i-1].StartTime.Before(klines[i].StartTime.Time()),
			"the result must be sorted by start time")
	}
}

// TestReadKLineDir_AcceptsArchives is a capability the function it replaces did
// not have: a downloaded cache can be read without unpacking it first.
func TestReadKLineDir_AcceptsArchives(t *testing.T) {
	dir := t.TempDir()

	content, err := os.ReadFile("testdata/um/BTCUSDT-1h-2026-09-15.csv")
	require.NoError(t, err)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("BTCUSDT-1h-2026-09-15.csv")
	require.NoError(t, err)
	_, err = io.Copy(w, bytes.NewReader(content))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	require.NoError(t, os.WriteFile(dir+"/a.zip", buf.Bytes(), 0o644))

	klines, err := ReadKLineDir(dir, "BTCUSDT", types.Interval1h)
	require.NoError(t, err)
	assert.Len(t, klines, 3)
}

// TestSource_TwoDatasetsStayOrdered is the regression test for a bug the
// original flat plan had: with aggTrades and klines configured on one source,
// reading every archive in date order emitted day two's trades before day one's
// hourly closes. Each series is now its own cursor and they are merged.
func TestSource_TwoDatasetsStayOrdered(t *testing.T) {
	srv := newFixtureServer()

	for offset, day := range []string{"2026-09-15", "2026-09-16"} {
		srv.serveShifted(t, aggTradeRef(t, day),
			"testdata/um/BTCUSDT-aggTrades-2026-09-15.csv", offset, 5)

		srv.serveShifted(t, FileRef{
			Market: MarketUSDMFutures, Period: PeriodDaily, Dataset: DatasetKLines,
			Symbol: "BTCUSDT", Interval: types.Interval1h, Date: date(t, day),
		}, "testdata/um/BTCUSDT-1h-2026-09-15.csv", offset, 0, 6)
	}

	src, err := New(Config{
		Market:    MarketUSDMFutures,
		Datasets:  []string{DatasetAggTrades, DatasetKLines},
		Intervals: []types.Interval{types.Interval1h},
		Cache:     srv.cache(t.TempDir()),
	})
	require.NoError(t, err)

	req := marketdata.Request{
		Since: date(t, "2026-09-15"),
		Until: date(t, "2026-09-17"),
		Subscriptions: []types.Subscription{
			{Symbol: "BTCUSDT", Channel: types.AggTradeChannel},
			{Symbol: "BTCUSDT", Channel: types.KLineChannel,
				Options: types.SubscribeOptions{Interval: types.Interval1h}},
		},
	}

	cur, err := src.Open(context.Background(), req)
	require.NoError(t, err)
	defer cur.Close()

	var prev marketdata.OrderKey
	var trades, klines int
	for cur.Next() {
		ev := cur.Event()
		assert.LessOrEqual(t, prev.Compare(ev.Key), 0,
			"two datasets in one source must still come out in time order")
		prev = ev.Key

		switch ev.Type {
		case marketdata.EventTypeTrade:
			trades++
		case marketdata.EventTypeKLine:
			klines++
		}
	}

	require.NoError(t, cur.Err())
	assert.Equal(t, 10, trades)
	assert.Equal(t, 6, klines)
}

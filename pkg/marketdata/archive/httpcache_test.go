package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/testing/httptesting"
)

const testURL = "https://data.binance.vision/data/futures/um/daily/aggTrades/BTCUSDT/BTCUSDT-aggTrades-2026-02-15.zip"

// buildZip returns an in-memory zip holding one CSV member, matching the shape
// data.binance.vision publishes.
func buildZip(t *testing.T, name, content string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	require.NoError(t, err)
	_, err = io.WriteString(w, content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// newCache wires an HTTPCache to a MockTransport serving the archive and its
// checksum sidecar, and reports how many archive requests were made.
func newCache(t *testing.T, dir string, payload []byte, checksum string) (*HTTPCache, *atomic.Int64) {
	t.Helper()

	var hits atomic.Int64
	transport := &httptesting.MockTransport{}

	path := "/data/futures/um/daily/aggTrades/BTCUSDT/BTCUSDT-aggTrades-2026-02-15.zip"

	transport.GET(path, func(req *http.Request) (*http.Response, error) {
		hits.Add(1)

		if rng := req.Header.Get("Range"); rng != "" {
			var offset int64
			if _, err := fmt.Sscanf(rng, "bytes=%d-", &offset); err != nil {
				return nil, err
			}
			if offset >= int64(len(payload)) {
				return httptesting.BuildResponse(http.StatusRequestedRangeNotSatisfiable, nil), nil
			}
			return httptesting.BuildResponse(http.StatusPartialContent, payload[offset:]), nil
		}

		return httptesting.BuildResponse(http.StatusOK, payload), nil
	})

	transport.GET(path+".CHECKSUM", func(req *http.Request) (*http.Response, error) {
		if checksum == "" {
			// what a publisher actually answers for an absent sidecar
			return httptesting.BuildResponse(http.StatusNotFound, nil), nil
		}
		body := checksum + "  BTCUSDT-aggTrades-2026-02-15.zip\n"
		return httptesting.BuildResponseString(http.StatusOK, body), nil
	})

	return &HTTPCache{
		Dir:        dir,
		HTTPClient: &http.Client{Transport: transport},
		Limiter:    rate.NewLimiter(rate.Inf, 1),
	}, &hits
}

func TestHTTPCache_LocalPathMirrorsPublisherLayout(t *testing.T) {
	c := &HTTPCache{Dir: "/cache"}

	got, err := c.LocalPath(testURL)
	require.NoError(t, err)

	assert.Equal(t,
		filepath.Join("/cache", "data.binance.vision", "data", "futures", "um", "daily",
			"aggTrades", "BTCUSDT", "BTCUSDT-aggTrades-2026-02-15.zip"),
		got)
}

func TestHTTPCache_LocalPathRejectsTraversal(t *testing.T) {
	c := &HTTPCache{Dir: "/cache"}

	// Cleaning resolves the traversal, so assert it cannot escape the root.
	got, err := c.LocalPath("https://evil.example/../../etc/passwd")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(got, filepath.Join("/cache", "evil.example")),
		"a cached path must stay under the cache root, got %s", got)
}

func TestHTTPCache_FetchAndCacheHit(t *testing.T) {
	dir := t.TempDir()
	payload := buildZip(t, "BTCUSDT-aggTrades-2026-02-15.csv", "1,2,3\n")

	c, hits := newCache(t, dir, payload, sha256Hex(payload))

	path, err := c.Fetch(context.Background(), testURL)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, payload, got, "the cache must store the original bytes")
	assert.Equal(t, int64(1), hits.Load())

	// second Fetch must not touch the network
	_, err = c.Fetch(context.Background(), testURL)
	require.NoError(t, err)
	assert.Equal(t, int64(1), hits.Load(), "a cache hit must issue no request")
}

func TestHTTPCache_ChecksumMismatchIsFatalAndUncached(t *testing.T) {
	dir := t.TempDir()
	payload := buildZip(t, "x.csv", "1,2,3\n")

	c, _ := newCache(t, dir, payload, sha256Hex([]byte("something else")))

	_, err := c.Fetch(context.Background(), testURL)
	require.Error(t, err)

	var mismatch *ChecksumError
	require.ErrorAs(t, err, &mismatch)

	path, err := c.LocalPath(testURL)
	require.NoError(t, err)
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr),
		"a file that failed verification must not stay in the cache")
}

func TestHTTPCache_ChecksumAbsent(t *testing.T) {
	dir := t.TempDir()
	payload := buildZip(t, "x.csv", "1,2,3\n")

	t.Run("if present, tolerated", func(t *testing.T) {
		c, _ := newCache(t, dir, payload, "")
		_, err := c.Fetch(context.Background(), testURL)
		assert.NoError(t, err)
	})

	t.Run("required, fatal", func(t *testing.T) {
		c, _ := newCache(t, t.TempDir(), payload, "")
		c.Verify = ChecksumRequired

		_, err := c.Fetch(context.Background(), testURL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no checksum published")
	})
}

// TestHTTPCache_ResumesPartialDownload checks that an interrupted run leaves a
// .part file that is resumed with a Range request rather than restarted, and
// never a truncated file the next run would treat as complete.
func TestHTTPCache_ResumesPartialDownload(t *testing.T) {
	dir := t.TempDir()
	payload := buildZip(t, "x.csv", strings.Repeat("1,2,3\n", 100))

	c, _ := newCache(t, dir, payload, sha256Hex(payload))

	path, err := c.LocalPath(testURL)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	const prefix = 32
	require.NoError(t, os.WriteFile(path+".part", payload[:prefix], 0o644))

	got, err := c.Fetch(context.Background(), testURL)
	require.NoError(t, err)

	content, err := os.ReadFile(got)
	require.NoError(t, err)
	assert.Equal(t, payload, content, "the resumed file must match the original exactly")

	_, statErr := os.Stat(path + ".part")
	assert.True(t, os.IsNotExist(statErr), "the .part file must be renamed away on success")
}

// TestHTTPCache_MissingArchive covers the failure the previous downloader
// swallowed: it logged and broke out of its loop, returning nil, so a 404 on
// day three of thirty silently produced a two-day dataset.
func TestHTTPCache_MissingArchive(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			transport := &httptesting.MockTransport{}
			transport.GET("/data/futures/um/daily/aggTrades/BTCUSDT/BTCUSDT-aggTrades-2026-02-15.zip",
				func(req *http.Request) (*http.Response, error) {
					return httptesting.BuildResponse(status, nil), nil
				})

			c := &HTTPCache{
				Dir:        t.TempDir(),
				HTTPClient: &http.Client{Transport: transport},
				Limiter:    rate.NewLimiter(rate.Inf, 1),
			}

			_, err := c.Fetch(context.Background(), testURL)
			require.Error(t, err)

			var missing *MissingArchiveError
			assert.ErrorAs(t, err, &missing,
				"an absent archive must be distinguishable from a transport failure")
		})
	}
}

func TestHTTPCache_RateLimiterIsHonoured(t *testing.T) {
	dir := t.TempDir()
	payload := buildZip(t, "x.csv", "1,2,3\n")

	c, _ := newCache(t, dir, payload, "")
	c.Limiter = rate.NewLimiter(rate.Limit(1000), 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Fetch(ctx, testURL)
	assert.Error(t, err, "a cancelled context must stop the limiter wait")
}

func TestOpen_Zip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.zip")

	content := "agg_trade_id,price\n1,100\n"
	require.NoError(t, os.WriteFile(path, buildZip(t, "x.csv", content), 0o644))

	entry, err := Open(path)
	require.NoError(t, err)
	defer entry.Close()

	got, err := io.ReadAll(entry)
	require.NoError(t, err)
	assert.Equal(t, content, string(got))
	assert.Equal(t, "x.csv", entry.Name)
}

func TestOpenCSV_StreamsThroughZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.zip")

	content := "agg_trade_id,price,quantity,first_trade_id,last_trade_id,transact_time,is_buyer_maker\n" +
		"3449899747,78153.0,0.001,8078264332,8078264332,1789430400003,false\n"
	require.NoError(t, os.WriteFile(path, buildZip(t, "x.csv", content), 0o644))

	r, closer, err := OpenCSV(path)
	require.NoError(t, err)
	defer closer.Close()

	rec, err := r.Read()
	require.NoError(t, err)
	assert.Equal(t, "3449899747", rec[0])

	idx, ok := r.Column("transact_time")
	require.True(t, ok)

	ts, err := ParseEpoch(rec[idx])
	require.NoError(t, err)
	assert.Equal(t, "2026-09-15T00:00:00.003Z", ts.Format("2006-01-02T15:04:05.999Z"))
}

func TestOpen_RejectsMultiMemberZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.zip")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"a.csv", "b.csv"} {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(w, "1\n")
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))

	_, err := Open(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than one member")
}

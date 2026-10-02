package archive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// ChecksumMode selects how downloaded archives are verified against the
// .CHECKSUM sidecar the publisher ships next to each file.
type ChecksumMode uint8

const (
	// ChecksumIfPresent verifies when a sidecar exists and skips when it does
	// not. This is the default: Binance publishes one per archive, other
	// venues do not.
	ChecksumIfPresent ChecksumMode = iota

	// ChecksumOff skips verification entirely.
	ChecksumOff

	// ChecksumRequired fails when no sidecar is available.
	ChecksumRequired
)

// MissingArchiveError reports that the publisher does not have the requested
// file. It is distinct from a transport failure so a caller can decide to
// tolerate a gap in coverage explicitly, rather than having a 404 silently
// truncate a range.
type MissingArchiveError struct {
	URL string
}

func (e *MissingArchiveError) Error() string {
	return fmt.Sprintf("archive: not published: %s", e.URL)
}

// ChecksumError reports that a downloaded file did not match its sidecar.
type ChecksumError struct {
	URL      string
	Expected string
	Actual   string
}

func (e *ChecksumError) Error() string {
	return fmt.Sprintf("archive: checksum mismatch for %s: expected %s, got %s",
		e.URL, e.Expected, e.Actual)
}

// HTTPCache downloads archives and caches them on disk, keyed by their URL path
// so the cache mirrors the publisher's layout. A cache built this way is
// self-describing, rsync-able, diffable against upstream, and — crucially —
// re-decodable, because it holds the original bytes rather than a normalized
// form.
type HTTPCache struct {
	// Dir is the cache root. A file from https://host/a/b.zip is stored at
	// <Dir>/host/a/b.zip.
	Dir string

	// HTTPClient defaults to a client with a 5 minute timeout, since these
	// archives can be hundreds of megabytes.
	HTTPClient *http.Client

	// Limiter throttles requests to the publisher. Defaults to 4 per second.
	Limiter *rate.Limiter

	// MaxRetries bounds retries of transient failures. A 404 is not retried.
	MaxRetries uint64

	// Verify selects checksum behaviour. The zero value verifies when a
	// sidecar is available.
	Verify ChecksumMode

	// VerifyOnHit re-verifies an already-cached file on every Fetch. Off by
	// default: hashing a large archive on every run is expensive and the
	// download path already verified it.
	VerifyOnHit bool

	Logger *log.Entry
}

func (c *HTTPCache) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (c *HTTPCache) limiter() *rate.Limiter {
	if c.Limiter != nil {
		return c.Limiter
	}
	c.Limiter = rate.NewLimiter(rate.Limit(4), 1)
	return c.Limiter
}

func (c *HTTPCache) logger() *log.Entry {
	if c.Logger != nil {
		return c.Logger
	}
	return log.WithField("component", "marketdata.archive")
}

func (c *HTTPCache) maxRetries() uint64 {
	if c.MaxRetries == 0 {
		return 3
	}
	return c.MaxRetries
}

// LocalPath returns where rawURL is cached, without touching the network.
func (c *HTTPCache) LocalPath(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("archive: bad url %q: %w", rawURL, err)
	}

	clean := filepath.Clean("/" + u.Path)
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("archive: refusing traversal in url path %q", u.Path)
	}

	return filepath.Join(c.Dir, u.Host, filepath.FromSlash(clean)), nil
}

// Fetch returns a local path for rawURL, downloading it if it is not cached.
//
// A download goes to <path>.part and is renamed only after it completes and
// verifies, so an interrupted run never leaves a truncated file that a later
// run would treat as a cache hit. An existing .part is resumed with a Range
// request.
func (c *HTTPCache) Fetch(ctx context.Context, rawURL string) (string, error) {
	path, err := c.LocalPath(rawURL)
	if err != nil {
		return "", err
	}

	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		if c.VerifyOnHit {
			if err := c.verify(ctx, rawURL, path); err != nil {
				return "", err
			}
		}
		return path, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	if err := c.download(ctx, rawURL, path); err != nil {
		return "", err
	}

	if err := c.verify(ctx, rawURL, path); err != nil {
		// A file that fails its checksum must not stay in the cache, or every
		// later run reuses the corruption.
		_ = os.Remove(path)
		return "", err
	}

	return path, nil
}

func (c *HTTPCache) download(ctx context.Context, rawURL, path string) error {
	partPath := path + ".part"

	op := func() error {
		offset := int64(0)
		if st, err := os.Stat(partPath); err == nil {
			offset = st.Size()
		}

		resp, err := c.get(ctx, rawURL, offset)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		flags := os.O_CREATE | os.O_WRONLY
		if resp.StatusCode == http.StatusPartialContent {
			flags |= os.O_APPEND
			c.logger().Debugf("resuming %s at offset %d", rawURL, offset)
		} else {
			flags |= os.O_TRUNC
		}

		f, err := os.OpenFile(partPath, flags, 0o644)
		if err != nil {
			return backoff.Permanent(err)
		}

		if _, err := io.Copy(f, resp.Body); err != nil {
			f.Close()
			// Keep the partial file: the next attempt resumes from it.
			return err
		}
		if err := f.Close(); err != nil {
			return backoff.Permanent(err)
		}

		return os.Rename(partPath, path)
	}

	bo := backoff.WithContext(
		backoff.WithMaxRetries(backoff.NewExponentialBackOff(), c.maxRetries()), ctx)

	return backoff.Retry(op, bo)
}

// get issues a single request, mapping a 404 to a permanent MissingArchiveError
// so it is never retried and never mistaken for a transport failure.
func (c *HTTPCache) get(ctx context.Context, rawURL string, offset int64) (*http.Response, error) {
	if err := c.limiter().Wait(ctx); err != nil {
		return nil, backoff.Permanent(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, backoff.Permanent(err)
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}

	switch {
	case resp.StatusCode == http.StatusOK, resp.StatusCode == http.StatusPartialContent:
		return resp, nil

	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden:
		// S3-backed publishers answer 403 for an absent key when listing is
		// denied, so both mean "not published".
		resp.Body.Close()
		return nil, backoff.Permanent(&MissingArchiveError{URL: rawURL})

	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		// The partial file is at least as long as the object; start over.
		resp.Body.Close()
		return nil, backoff.Permanent(fmt.Errorf("archive: stale partial download for %s", rawURL))

	default:
		resp.Body.Close()
		return nil, fmt.Errorf("archive: unexpected status %d for %s", resp.StatusCode, rawURL)
	}
}

// verify checks path against the publisher's .CHECKSUM sidecar.
func (c *HTTPCache) verify(ctx context.Context, rawURL, path string) error {
	if c.Verify == ChecksumOff {
		return nil
	}

	expected, err := c.fetchChecksum(ctx, rawURL)
	if err != nil {
		var missing *MissingArchiveError
		if errors.As(err, &missing) {
			if c.Verify == ChecksumRequired {
				return fmt.Errorf("archive: no checksum published for %s", rawURL)
			}
			return nil
		}
		return err
	}

	actual, err := sha256File(path)
	if err != nil {
		return err
	}

	if !strings.EqualFold(actual, expected) {
		return &ChecksumError{URL: rawURL, Expected: expected, Actual: actual}
	}

	return nil
}

func (c *HTTPCache) fetchChecksum(ctx context.Context, rawURL string) (string, error) {
	resp, err := c.get(ctx, rawURL+".CHECKSUM", 0)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// The sidecar is one short line: "<sha256>  <filename>".
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}

	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", fmt.Errorf("archive: empty checksum file for %s", rawURL)
	}

	return fields[0], nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

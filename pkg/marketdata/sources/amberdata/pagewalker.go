package amberdata

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/marketdata/sources/amberdata/amberdataapi"
)

// pageWalker fetches a time range as a sequence of pages.
//
// It chunks the range, then follows the cursor within each chunk, and halves the
// chunk when the server says the result is too large or takes too long. The 10 MB
// response cap, not the time window, is usually what binds on trades and order
// book pulls, so reacting to it by shrinking rather than retrying is the whole
// point.
//
// pkg/exchange/batch.AsyncTimeRangedBatchQuery solves a similar problem and was
// considered, but is the wrong tool here: it is reflection-based throughout, it
// accumulates a deduplication map over the entire range — tens of millions of
// keys for a month of tick data — and it advances its window to the last emitted
// timestamp, which silently drops any event sharing that timestamp. Its ideas
// worth keeping, a limiter and a backoff and jumping empty windows, are cheap to
// reproduce typed.
type pageWalker[T any] struct {
	// fetch requests one chunk. next is empty for the first page of a chunk and
	// a cursor URL afterwards.
	fetch func(ctx context.Context, since, until time.Time, next string) (amberdataapi.Page[T], error)

	// timeOf and idOf identify a record, for the deduplication described below.
	timeOf func(T) int64
	idOf   func(T) string

	limiter *rate.Limiter

	// chunk is the initial window, minChunk the floor below which shrinking
	// gives up and reports the error.
	chunk    time.Duration
	minChunk time.Duration

	// maxWindow is the endpoint's documented limit, which caps chunk.
	maxWindow time.Duration

	logger *log.Entry

	// lastTimeNs and lastIDs deduplicate across a chunk boundary, bounded to the
	// identifiers seen at the single most recent timestamp. Anything larger is
	// what makes the batch helper unusable at tick volume.
	lastTimeNs int64
	lastIDs    map[string]struct{}
}

func (w *pageWalker[T]) init() {
	if w.chunk <= 0 {
		w.chunk = time.Hour
	}
	if w.minChunk <= 0 {
		w.minChunk = time.Minute
	}
	if w.maxWindow > 0 && w.chunk > w.maxWindow {
		w.chunk = w.maxWindow
	}
	if w.limiter == nil {
		w.limiter = rate.NewLimiter(rate.Limit(10), 1)
	}
	if w.logger == nil {
		w.logger = log.WithField("component", "amberdata.pagewalker")
	}
	if w.lastIDs == nil {
		w.lastIDs = map[string]struct{}{}
	}
}

// walk calls emit for every record in [since, until), in order. emit returning
// false stops the walk cleanly.
func (w *pageWalker[T]) walk(
	ctx context.Context, since, until time.Time, emit func(T) bool,
) error {
	w.init()

	chunk := w.chunk

	for cursorTime := since; cursorTime.Before(until); {
		chunkEnd := cursorTime.Add(chunk)
		if chunkEnd.After(until) {
			chunkEnd = until
		}

		page, err := w.fetchChunk(ctx, cursorTime, chunkEnd)
		if err != nil {
			var apiErr *amberdataapi.APIError
			if errors.As(err, &apiErr) && apiErr.ShouldShrinkWindow() {
				// Halve the window that actually failed, not the nominal chunk.
				// The chunk is clamped to the end of the range, so a 24 hour
				// chunk over a 30 minute range fails at 30 minutes; halving the
				// chunk would then retry the identical request several times
				// before having any effect.
				effective := chunkEnd.Sub(cursorTime)
				if effective <= w.minChunk {
					return fmt.Errorf(
						"amberdata: %s..%s is too large even at the minimum chunk of %s: %w",
						cursorTime.Format(time.RFC3339), chunkEnd.Format(time.RFC3339),
						w.minChunk, err)
				}

				chunk = effective / 2
				if chunk < w.minChunk {
					chunk = w.minChunk
				}
				w.logger.WithError(err).Infof("shrinking the request window to %s", chunk)
				continue
			}
			return err
		}

		for {
			for _, record := range page.Data {
				if w.seen(record) {
					continue
				}
				if !emit(record) {
					return nil
				}
			}

			if page.Next == "" {
				break
			}

			page, err = w.fetchCursor(ctx, page.Next)
			if err != nil {
				return err
			}
		}

		cursorTime = chunkEnd
	}

	return nil
}

func (w *pageWalker[T]) fetchChunk(
	ctx context.Context, since, until time.Time,
) (amberdataapi.Page[T], error) {
	if err := w.limiter.Wait(ctx); err != nil {
		return amberdataapi.Page[T]{}, err
	}
	return w.fetch(ctx, since, until, "")
}

func (w *pageWalker[T]) fetchCursor(
	ctx context.Context, cursorURL string,
) (amberdataapi.Page[T], error) {
	if err := w.limiter.Wait(ctx); err != nil {
		return amberdataapi.Page[T]{}, err
	}
	return w.fetch(ctx, time.Time{}, time.Time{}, cursorURL)
}

// seen reports whether a record was already emitted.
//
// Chunk boundaries are inclusive on one side, so a record sitting exactly on one
// can arrive twice. Only the identifiers at the most recent timestamp need
// remembering to catch that, which keeps the memory cost proportional to the
// number of records sharing one timestamp rather than to the whole range.
func (w *pageWalker[T]) seen(record T) bool {
	ts := w.timeOf(record)

	if ts != w.lastTimeNs {
		w.lastTimeNs = ts
		clear(w.lastIDs)
	}

	id := w.idOf(record)
	if id == "" {
		return false
	}

	if _, dup := w.lastIDs[id]; dup {
		return true
	}

	w.lastIDs[id] = struct{}{}
	return false
}

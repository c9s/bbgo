package binancecsv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/archive"
)

// batchSize bounds how many events are decoded before they are handed out. It
// caps peak memory at one batch rather than one day of ticks, which matters:
// a day of BTCUSDT aggregated trades is tens of millions of records.
const batchSize = 4096

// seriesCursor streams one series — a single dataset, symbol and interval — by
// reading its archives in date order.
//
// A series is monotonic by construction, because each archive covers a distinct
// day or month and its records are published in order. That is why a series,
// and not the whole plan, is the unit here: a source configured with both
// aggTrades and klines has two series whose files interleave in time, and
// reading them as one flat date-ordered list would emit day 2's trades before
// day 1's hourly closes. Source.Open therefore builds one seriesCursor per
// series and merges them with marketdata.Merge.
//
// Within a series only one archive is open at a time, closed as soon as it is
// exhausted, which is what keeps a multi-hundred-file range from exhausting
// file descriptors.
type seriesCursor struct {
	ctx    context.Context
	source *Source
	tasks  []fileTask
	req    marketdata.Request

	taskIdx int
	reader  *archive.Reader
	closer  io.Closer
	current fileTask

	batch []marketdata.Event
	pos   int
	cur   *marketdata.Event

	err    error
	closed bool
	logger *log.Entry
}

func newSeriesCursor(
	ctx context.Context, s *Source, tasks []fileTask, req marketdata.Request,
) *seriesCursor {
	return &seriesCursor{
		ctx:     ctx,
		source:  s,
		tasks:   tasks,
		req:     req,
		batch:   make([]marketdata.Event, 0, batchSize),
		logger:  log.WithField("component", s.Name()),
		taskIdx: -1,
	}
}

func (c *seriesCursor) Next() bool {
	if c.err != nil || c.closed {
		c.cur = nil
		return false
	}

	for {
		if c.pos < len(c.batch) {
			c.cur = &c.batch[c.pos]
			c.pos++
			return true
		}

		if !c.fillBatch() {
			c.cur = nil
			return false
		}
	}
}

// fillBatch decodes the next batch of events, advancing to the next archive
// when the current one is exhausted. It reports whether anything was produced.
func (c *seriesCursor) fillBatch() bool {
	for {
		if err := c.ctx.Err(); err != nil {
			c.err = err
			return false
		}

		if c.reader == nil {
			if !c.openNextFile() {
				return false
			}
		}

		c.batch = c.batch[:0]
		c.pos = 0

		for len(c.batch) < batchSize {
			record, err := c.reader.Read()
			if err == io.EOF {
				c.closeCurrentFile()
				break
			}
			if err != nil {
				c.err = fmt.Errorf("binancecsv: reading %s: %w", c.current.ref.FileName(), err)
				return false
			}

			meta := c.current.meta
			meta.Columns = c.reader.Columns()
			meta.LineNo = c.reader.Line()

			before := len(c.batch)
			decoded, err := c.current.decoder.Decode(c.batch, record, meta)
			if err != nil {
				c.err = fmt.Errorf("binancecsv: %s line %d: %w",
					c.current.ref.FileName(), meta.LineNo, err)
				return false
			}

			// An archive covers a whole UTC day or month, but a request may
			// start or end part-way through it, so events outside the range are
			// dropped here rather than leaking into the merge.
			c.batch = decoded[:before]
			for i := before; i < len(decoded); i++ {
				if c.inRange(decoded[i].Key.TimeNs) {
					c.batch = append(c.batch, decoded[i])
				}
			}
		}

		// Records inside one archive are published in order, but a decoder may
		// fan one record out into several events with different ranks, and the
		// last batch of a file can interleave with the next file only if the
		// archives overlap. Sorting the batch keeps the cursor's ordering
		// guarantee cheaply and locally.
		sort.SliceStable(c.batch, func(i, j int) bool {
			return c.batch[i].Key.Compare(c.batch[j].Key) < 0
		})

		if len(c.batch) > 0 {
			return true
		}

		if c.reader == nil && c.taskIdx >= len(c.tasks)-1 {
			return false
		}
	}
}

// openNextFile fetches and opens the next archive, skipping ones that fall
// outside the requested range and, when configured, ones the publisher does
// not have.
func (c *seriesCursor) openNextFile() bool {
	for {
		c.taskIdx++
		if c.taskIdx >= len(c.tasks) {
			return false
		}

		task := c.tasks[c.taskIdx]

		// An archive covering a period that ends at or before Since, or starts
		// at or after Until, contributes nothing.
		if !c.source.periodEnd(task.ref).After(c.req.Since) || !task.ref.Date.Before(c.req.Until) {
			continue
		}

		path, err := c.source.cache.Fetch(c.ctx, task.ref.URL())
		if err != nil {
			if isMissing(err) && c.source.cfg.AllowMissingDays {
				c.logger.Warnf("archive not published, skipping: %s", task.ref.FileName())
				continue
			}
			c.err = err
			return false
		}

		reader, closer, err := archive.OpenCSV(path)
		if err != nil {
			c.err = err
			return false
		}

		c.reader, c.closer, c.current = reader, closer, task
		return true
	}
}

func (c *seriesCursor) closeCurrentFile() {
	if c.closer != nil {
		if err := c.closer.Close(); err != nil {
			c.logger.WithError(err).Warnf("closing %s", c.current.ref.FileName())
		}
	}
	c.reader, c.closer = nil, nil
}

func (c *seriesCursor) Event() *marketdata.Event { return c.cur }
func (c *seriesCursor) Err() error               { return c.err }

func (c *seriesCursor) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	c.cur = nil
	c.closeCurrentFile()
	return nil
}

// isMissing reports whether err means the publisher has no such archive, as
// opposed to a transport failure.
func isMissing(err error) bool {
	var missing *archive.MissingArchiveError
	return errors.As(err, &missing)
}

// inRange reports whether an event time falls inside the requested half-open
// range. Decoders stay ignorant of the range; the filter lives here.
func (c *seriesCursor) inRange(timeNs int64) bool {
	return timeNs >= c.req.Since.UnixNano() && timeNs < c.req.Until.UnixNano()
}

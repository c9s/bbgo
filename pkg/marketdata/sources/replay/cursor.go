package replay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// cursor replays recordings in file order.
//
// Recordings are named by their UTC hour and hold records in the order they
// arrived, so reading the files in name order yields a non-decreasing stream
// with no buffering and no sort.
type cursor struct {
	ctx      context.Context
	files    []string
	req      marketdata.Request
	symbols  []string
	channels []types.Channel

	fileIdx int
	header  Header
	dec     *json.Decoder
	closer  io.Closer

	cur    *marketdata.Event
	event  marketdata.Event
	err    error
	closed bool
	logger *log.Entry
}

func (c *cursor) Next() bool {
	if c.err != nil || c.closed {
		c.cur = nil
		return false
	}

	for {
		if err := c.ctx.Err(); err != nil {
			c.err = err
			c.cur = nil
			return false
		}

		if c.dec == nil {
			if !c.openNextFile() {
				c.cur = nil
				return false
			}
		}

		var record Record
		if err := c.dec.Decode(&record); err != nil {
			if errors.Is(err, io.EOF) {
				c.closeCurrentFile()
				continue
			}
			c.err = fmt.Errorf("replay: reading %s: %w",
				filepath.Base(c.files[c.fileIdx]), err)
			c.cur = nil
			return false
		}

		if !c.wanted(record) {
			continue
		}

		event, err := DecodeRecord(record, c.header)
		if err != nil {
			c.err = err
			c.cur = nil
			return false
		}

		c.event = event
		c.cur = &c.event
		return true
	}
}

// wanted filters a record against the request's range, symbols and channels
// before it is decoded, which keeps the common case cheap.
func (c *cursor) wanted(r Record) bool {
	if r.T < c.req.Since.UnixNano() || r.T >= c.req.Until.UnixNano() {
		return false
	}

	symbol := r.S
	if symbol == "" && len(c.header.Symbols) == 1 {
		symbol = c.header.Symbols[0]
	}
	if len(c.symbols) > 0 && symbol != "" && !slices.Contains(c.symbols, symbol) {
		return false
	}

	if len(c.channels) > 0 && !slices.Contains(c.channels, channelOf(r.Ty)) {
		return false
	}

	return true
}

// channelOf maps a recorded event type back to the stream channel a
// subscription would name.
func channelOf(t marketdata.EventType) types.Channel {
	switch t {
	case marketdata.EventTypeBookSnapshot, marketdata.EventTypeBookUpdate:
		return types.BookChannel
	case marketdata.EventTypeTrade:
		return types.MarketTradeChannel
	case marketdata.EventTypeBookTicker:
		return types.BookTickerChannel
	case marketdata.EventTypeKLine:
		return types.KLineChannel
	default:
		return ""
	}
}

func (c *cursor) openNextFile() bool {
	for {
		c.fileIdx++
		if c.fileIdx >= len(c.files) {
			return false
		}

		header, closer, dec, err := openRecording(c.files[c.fileIdx])
		if err != nil {
			c.err = err
			return false
		}

		c.header, c.closer, c.dec = header, closer, dec
		return true
	}
}

func (c *cursor) closeCurrentFile() {
	if c.closer != nil {
		if err := c.closer.Close(); err != nil {
			c.logger.WithError(err).Warnf("closing %s", filepath.Base(c.files[c.fileIdx]))
		}
	}
	c.closer, c.dec = nil, nil
}

func (c *cursor) Event() *marketdata.Event { return c.cur }
func (c *cursor) Err() error               { return c.err }

func (c *cursor) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	c.cur = nil
	c.closeCurrentFile()
	return nil
}

package mdtest

import (
	"context"
	"testing"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/types"
)

// CountingCursor wraps a cursor and records how often Close was called, so a
// test can assert the merge closes every input exactly once.
type CountingCursor struct {
	marketdata.Cursor
	Closes int
}

func NewCountingCursor(c marketdata.Cursor) *CountingCursor {
	return &CountingCursor{Cursor: c}
}

func (c *CountingCursor) Close() error {
	c.Closes++
	return c.Cursor.Close()
}

// ErrCursor emits events and then fails with Err, so error-policy paths can be
// exercised without a real provider.
type ErrCursor struct {
	Events []marketdata.Event
	Fail   error

	pos int
	cur *marketdata.Event
}

func (c *ErrCursor) Next() bool {
	if c.pos >= len(c.Events) {
		c.cur = nil
		return false
	}
	c.cur = &c.Events[c.pos]
	c.pos++
	return true
}

func (c *ErrCursor) Event() *marketdata.Event { return c.cur }

// Err reports the failure once the events are exhausted, matching how a real
// cursor surfaces a mid-stream read error.
func (c *ErrCursor) Err() error {
	if c.pos >= len(c.Events) {
		return c.Fail
	}
	return nil
}

func (c *ErrCursor) Close() error { return nil }

// StaticSource is a Source backed by a fixed slice of events. It serves the
// registry and merge tests, and gives providers a reference for the Source
// contract.
type StaticSource struct {
	SourceName string
	Events     []marketdata.Event
	Capability marketdata.Capability
	OpenErr    error
}

// NewStaticSource returns a source named name that serves events, declaring a
// capability wide enough to accept any request.
func NewStaticSource(name string, events []marketdata.Event) *StaticSource {
	return &StaticSource{
		SourceName: name,
		Events:     events,
		Capability: marketdata.Capability{
			Channels: []types.Channel{
				types.KLineChannel,
				types.MarketTradeChannel,
				types.AggTradeChannel,
				types.BookChannel,
				types.BookTickerChannel,
			},
			HasHistory: true,
		},
	}
}

func (s *StaticSource) Name() string                        { return s.SourceName }
func (s *StaticSource) Capabilities() marketdata.Capability { return s.Capability }

func (s *StaticSource) Open(ctx context.Context, req marketdata.Request) (marketdata.Cursor, error) {
	if s.OpenErr != nil {
		return nil, s.OpenErr
	}
	if err := s.Capability.Validate(s.SourceName, req); err != nil {
		return nil, err
	}

	events := make([]marketdata.Event, len(s.Events))
	copy(events, s.Events)
	return marketdata.NewSliceCursor(events), nil
}

// Collect drains a cursor into a slice, failing the test on error. Events are
// cloned, since a Cursor only guarantees its event until the next Next call.
func Collect(t *testing.T, c marketdata.Cursor) []*marketdata.Event {
	t.Helper()

	var out []*marketdata.Event
	for c.Next() {
		out = append(out, c.Event().Clone())
	}
	if err := c.Err(); err != nil {
		t.Fatalf("cursor error: %v", err)
	}
	return out
}

// Keys returns the ordering keys of events, for compact assertions.
func Keys(events []*marketdata.Event) []marketdata.OrderKey {
	out := make([]marketdata.OrderKey, len(events))
	for i, ev := range events {
		out[i] = ev.Key
	}
	return out
}

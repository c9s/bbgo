package marketdata

import "sort"

// SliceCursor is a Cursor over an in-memory slice of events.
//
// It exists for two callers: providers that must buffer anyway (a REST page, a
// decoded CSV chunk) and tests. It sorts on construction, so a provider that
// receives a page in an unspecified order satisfies the Cursor ordering
// guarantee for free.
type SliceCursor struct {
	events []Event
	pos    int
	cur    *Event
	closed bool
}

// NewSliceCursor returns a cursor over events, sorting them by OrderKey.
// It takes ownership of the slice.
func NewSliceCursor(events []Event) *SliceCursor {
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Key.Compare(events[j].Key) < 0
	})
	return &SliceCursor{events: events}
}

// NewSortedSliceCursor returns a cursor over events that are already known to
// be sorted, skipping the sort. Use Validate to assert that in tests.
func NewSortedSliceCursor(events []Event) *SliceCursor {
	return &SliceCursor{events: events}
}

func (c *SliceCursor) Next() bool {
	if c.closed || c.pos >= len(c.events) {
		c.cur = nil
		return false
	}
	c.cur = &c.events[c.pos]
	c.pos++
	return true
}

func (c *SliceCursor) Event() *Event { return c.cur }
func (c *SliceCursor) Err() error    { return nil }

func (c *SliceCursor) Close() error {
	c.closed = true
	c.cur = nil
	return nil
}

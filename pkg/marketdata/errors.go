package marketdata

import (
	"errors"
	"fmt"
	"strings"

	"github.com/c9s/bbgo/pkg/types"
)

var (
	// ErrOutOfOrder is returned when a cursor emits an event that sorts before
	// the one it emitted previously, violating the Cursor contract.
	ErrOutOfOrder = errors.New("marketdata: events out of order")

	// ErrOverflow is returned by a ChanCursor running under OverflowFail whose
	// buffer filled up.
	ErrOverflow = errors.New("marketdata: event buffer overflow")

	// ErrClosed is returned when an operation is attempted on a closed cursor.
	ErrClosed = errors.New("marketdata: cursor is closed")

	// ErrBookNotReady is returned by BookState.Apply for an update that arrives
	// before any snapshot has initialized the book.
	ErrBookNotReady = errors.New("marketdata: order book has no snapshot yet")

	// ErrBookGap is returned by BookState.Apply when the sequence numbers show
	// a missing update under the configured SequenceMode.
	ErrBookGap = errors.New("marketdata: order book sequence gap")

	// ErrBookCrossed is returned by BookState.Apply when applying an event
	// leaves the book crossed or otherwise invalid.
	ErrBookCrossed = errors.New("marketdata: order book is crossed")
)

// UnsupportedError reports that a Source cannot serve part of a Request. A
// Source must return this from Open rather than silently producing no events.
type UnsupportedError struct {
	Source string
	Reason string

	// Channel, Symbol and Interval identify the offending subscription when the
	// rejection is about one. They may be empty.
	Channel  types.Channel
	Symbol   string
	Interval types.Interval
}

func (e *UnsupportedError) Error() string {
	var sb strings.Builder
	sb.WriteString("marketdata: source ")
	sb.WriteString(e.Source)
	sb.WriteString(" cannot serve")

	if len(e.Channel) > 0 {
		fmt.Fprintf(&sb, " channel %s", e.Channel)
	}
	if len(e.Symbol) > 0 {
		fmt.Fprintf(&sb, " symbol %s", e.Symbol)
	}
	if len(e.Interval) > 0 {
		fmt.Fprintf(&sb, " interval %s", e.Interval)
	}
	if len(e.Reason) > 0 {
		sb.WriteString(": ")
		sb.WriteString(e.Reason)
	}

	return sb.String()
}

// SourceError wraps an error from one input of a merge, naming the source so
// the failure is attributable.
type SourceError struct {
	Source string
	Index  int32
	Err    error
}

func (e *SourceError) Error() string {
	return fmt.Sprintf("marketdata: source %s (index %d): %v", e.Source, e.Index, e.Err)
}

func (e *SourceError) Unwrap() error { return e.Err }

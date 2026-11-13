package marketdata

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// Request describes the data a Source should produce.
type Request struct {
	// Since is inclusive, Until is exclusive. Both must be non-zero.
	Since time.Time
	Until time.Time

	// Subscriptions selects channels, symbols and intervals. It reuses
	// types.Subscription so that the backtest engine can later pass
	// MarketDataStream.GetSubscriptions() through unchanged.
	Subscriptions []types.Subscription
}

// Validate checks that the request is self-consistent.
func (r Request) Validate() error {
	if r.Since.IsZero() || r.Until.IsZero() {
		return fmt.Errorf("marketdata: request needs both Since and Until")
	}
	if !r.Until.After(r.Since) {
		return fmt.Errorf("marketdata: request Until (%s) must be after Since (%s)",
			r.Until.Format(time.RFC3339), r.Since.Format(time.RFC3339))
	}
	if len(r.Subscriptions) == 0 {
		return fmt.Errorf("marketdata: request needs at least one subscription")
	}
	return nil
}

// Symbols returns the distinct symbols in the request's subscriptions.
func (r Request) Symbols() []string {
	var out []string
	for _, sub := range r.Subscriptions {
		if !slices.Contains(out, sub.Symbol) {
			out = append(out, sub.Symbol)
		}
	}
	return out
}

// Capability declares what a Source can serve. Open must reject a Request that
// Capability does not cover, with an error — never by silently emitting nothing.
type Capability struct {
	// Exchanges, Channels, Intervals and Symbols are allow-lists. A nil slice
	// means "no restriction on this dimension", except Channels, where nil
	// means the source serves nothing.
	Exchanges []types.ExchangeName
	Channels  []types.Channel
	Intervals []types.Interval // meaningful only when Channels contains KLineChannel
	Symbols   []string

	// HasHistory reports whether the source can serve a bounded [Since, Until)
	// range. A live-only source sets this false; Merge refuses to include it.
	HasHistory bool

	// CoverageStart and CoverageEnd, when non-zero, bound the historical range
	// the source has data for. They exist so Open can fail with a message
	// naming the real window instead of returning an empty cursor.
	CoverageStart time.Time
	CoverageEnd   time.Time
}

// Supports reports whether the capability covers one subscription.
func (c Capability) Supports(sub types.Subscription) bool {
	if !slices.Contains(c.Channels, sub.Channel) {
		return false
	}
	if c.Symbols != nil && !slices.Contains(c.Symbols, sub.Symbol) {
		return false
	}
	if sub.Channel == types.KLineChannel && c.Intervals != nil {
		if !slices.Contains(c.Intervals, sub.Options.Interval) {
			return false
		}
	}
	return true
}

// Covered returns the subscriptions this capability can serve, and Uncovered
// returns the rest.
//
// A source is not required to serve every subscription in a request: in a merge
// each source contributes what it has. Whether the request as a whole is
// covered is a question about the set of sources, so MergeSources answers it,
// not the individual source.
func (c Capability) Covered(req Request) []types.Subscription {
	var out []types.Subscription
	for _, sub := range req.Subscriptions {
		if c.Supports(sub) {
			out = append(out, sub)
		}
	}
	return out
}

func (c Capability) Uncovered(req Request) []types.Subscription {
	var out []types.Subscription
	for _, sub := range req.Subscriptions {
		if !c.Supports(sub) {
			out = append(out, sub)
		}
	}
	return out
}

// Validate checks a request against the capability and returns an
// *UnsupportedError describing the first problem it finds.
//
// It rejects a request only when the source can contribute nothing to it, or
// when the source cannot serve a bounded range at all. A source that covers
// some of the subscriptions is opened and serves those.
func (c Capability) Validate(sourceName string, req Request) error {
	if !c.HasHistory {
		return &UnsupportedError{
			Source: sourceName,
			Reason: "source is live-only and cannot serve a bounded time range",
		}
	}

	if len(c.Covered(req)) == 0 {
		return &UnsupportedError{
			Source: sourceName,
			Reason: fmt.Sprintf("none of the requested subscriptions are served; this source serves channels %v",
				c.Channels),
		}
	}

	if !c.CoverageStart.IsZero() && req.Since.Before(c.CoverageStart) {
		return &UnsupportedError{
			Source: sourceName,
			Reason: fmt.Sprintf("data starts at %s, requested from %s",
				c.CoverageStart.Format(time.RFC3339), req.Since.Format(time.RFC3339)),
		}
	}
	if !c.CoverageEnd.IsZero() && req.Until.After(c.CoverageEnd) {
		return &UnsupportedError{
			Source: sourceName,
			Reason: fmt.Sprintf("data ends at %s, requested until %s",
				c.CoverageEnd.Format(time.RFC3339), req.Until.Format(time.RFC3339)),
		}
	}

	return nil
}

// Source opens time-ranged cursors over one origin of market data. A Source is
// reusable and safe for concurrent Open calls; the Cursors it returns are not.
type Source interface {
	// Name is a stable identifier used in logs, in Event.Source, and for the
	// merge's deterministic source ordering.
	Name() string

	// Capabilities declares what this source can serve.
	Capabilities() Capability

	// Open validates req and returns a Cursor positioned before the first
	// event. The Cursor is bound to ctx: cancelling ctx makes Next return false
	// with Err reporting ctx.Err().
	Open(ctx context.Context, req Request) (Cursor, error)
}

// Cursor is a pull-based, forward-only iterator over time-ordered events.
//
// An implementation must guarantee:
//
//   - Events are non-decreasing in OrderKey, ignoring SourceIndex. A cursor
//     that cannot guarantee this must buffer and sort internally.
//   - Event returns a pointer valid only until the next Next call; the payload
//     may be reused.
//   - Next returns false exactly once at exhaustion; further calls return false.
//   - Err returns nil on clean exhaustion and non-nil on failure. Once non-nil
//     it stays non-nil and the cursor is dead.
//   - Close is idempotent, releases every resource including background
//     goroutines, and is safe to call before Next has ever been called.
//
// A Cursor is NOT safe for concurrent use.
type Cursor interface {
	Next() bool
	Event() *Event
	Err() error
	Close() error
}

// NamedCursor pairs a cursor with the name of the source that produced it, so
// the merge can stamp Event.Source and attribute errors.
type NamedCursor struct {
	Name   string
	Cursor Cursor
}

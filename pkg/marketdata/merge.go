package marketdata

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/multierr"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

// ErrorPolicy decides what a merge does when one of its inputs fails.
type ErrorPolicy uint8

const (
	// FailFast is the default: the first input error closes every cursor and
	// surfaces via Err. A truncated backtest feed produces a plausible-looking
	// but wrong result, which is worse than a failed run.
	FailFast ErrorPolicy = iota

	// SkipSource logs the failing input, drops it, and continues with the rest.
	// For exploratory runs only; never for a reported backtest.
	SkipSource
)

// SourceStats summarizes what one input contributed to a merge. It is the
// diagnostic that turns "my backtest looks wrong" into "source 2 only covered
// 3 of the 30 days".
type SourceStats struct {
	Name  string
	Index int32
	Count int64
	First time.Time
	Last  time.Time
	Err   error
}

type mergeItem struct {
	name  string
	cur   Cursor
	ev    *Event
	index int32

	count int64
	first time.Time
	last  time.Time
	err   error

	// live means "has more events"; closed means "the underlying cursor has
	// been released". They are distinct: a cursor that errors stops being live
	// but still has to be closed.
	live   bool
	closed bool
}

// mergeHeap orders items by their head event's OrderKey. The key already
// carries SourceIndex, stamped when the item was primed, so ties are broken
// deterministically without a secondary comparison here.
type mergeHeap []*mergeItem

func (h mergeHeap) Len() int           { return len(h) }
func (h mergeHeap) Less(i, j int) bool { return h[i].ev.Key.Compare(h[j].ev.Key) < 0 }
func (h mergeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *mergeHeap) Push(x any) { *h = append(*h, x.(*mergeItem)) }

func (h *mergeHeap) Pop() any {
	old := *h
	n := len(old)
	it := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return it
}

// MergeCursor interleaves k cursors into a single non-decreasing stream.
//
// Complexity is O(n log k) comparisons for n total events and O(k) resident
// memory beyond whatever each input buffers internally: exactly one event per
// input is held at a time. There is no global buffer and no global sort, which
// is what makes a multi-year, multi-source tick replay possible at all.
type MergeCursor struct {
	items []*mergeItem // stable, indexed by SourceIndex
	h     mergeHeap

	cur     *Event
	pending *mergeItem // the item whose head we just handed out

	last    OrderKey
	hasLast bool

	err    error
	closed bool

	policy ErrorPolicy
	logger *log.Entry
}

// MergeOption configures a MergeCursor.
type MergeOption func(*MergeCursor)

// WithErrorPolicy sets how input errors are handled. The default is FailFast.
func WithErrorPolicy(p ErrorPolicy) MergeOption {
	return func(m *MergeCursor) { m.policy = p }
}

// WithMergeLogger sets the logger used for diagnostics.
func WithMergeLogger(l *log.Entry) MergeOption {
	return func(m *MergeCursor) { m.logger = l }
}

// Merge interleaves cursors into a single time-ordered cursor.
//
// Merge takes ownership: Close closes every input exactly once, including on
// the error paths. Inputs are sorted by name before an index is assigned, so
// the tie-break — and therefore the output — is reproducible across runs
// regardless of map iteration order upstream.
func Merge(cursors []NamedCursor, opts ...MergeOption) *MergeCursor {
	m := &MergeCursor{
		logger: log.WithField("component", "marketdata.merge"),
	}
	for _, opt := range opts {
		opt(m)
	}

	sorted := make([]NamedCursor, len(cursors))
	copy(sorted, cursors)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	m.items = make([]*mergeItem, len(sorted))
	for i, nc := range sorted {
		m.items[i] = &mergeItem{
			name:  nc.Name,
			cur:   nc.Cursor,
			index: int32(i),
			live:  true,
		}
	}

	// Prime every input with its first event. An input that fails here is
	// handled exactly as one that fails mid-stream, so an unreadable source
	// cannot masquerade as an empty one.
	m.h = make(mergeHeap, 0, len(m.items))
	for _, it := range m.items {
		if m.advance(it) {
			m.h = append(m.h, it)
			continue
		}

		if it.err != nil {
			srcErr := &SourceError{Source: it.name, Index: it.index, Err: it.err}
			if m.policy == FailFast && m.err == nil {
				m.err = srcErr
			} else {
				m.logger.WithError(srcErr).Warnf("dropping market data source %s", it.name)
			}
		}

		m.closeItem(it)
	}

	if m.err != nil {
		m.closeAll()
		return m
	}

	heap.Init(&m.h)

	return m
}

// MergeSources opens every source over the same request and merges the result.
// If any Open fails it closes whatever it already opened and returns the error.
//
// It first checks that the sources together cover every subscription. An
// individual source only serves what it has, so this is the only place with
// enough information to notice that nobody serves, say, the order book — and
// noticing matters, because the alternative is a backtest that silently runs
// without the data a strategy asked for.
func MergeSources(
	ctx context.Context, sources []Source, req Request, opts ...MergeOption,
) (*MergeCursor, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if err := CheckCoverage(sources, req); err != nil {
		return nil, err
	}

	var opened []NamedCursor
	for _, src := range sources {
		cur, err := src.Open(ctx, req)
		if err != nil {
			var closeErr error
			for _, nc := range opened {
				closeErr = multierr.Append(closeErr, nc.Cursor.Close())
			}
			return nil, multierr.Append(fmt.Errorf("opening source %s: %w", src.Name(), err), closeErr)
		}
		opened = append(opened, NamedCursor{Name: src.Name(), Cursor: cur})
	}

	return Merge(opened, opts...), nil
}

// advance pulls the next event from it, stamping the ordering and provenance
// fields the merge owns. It reports whether an event is now available.
func (m *MergeCursor) advance(it *mergeItem) bool {
	if !it.live {
		return false
	}

	if it.cur.Next() {
		ev := it.cur.Event()
		ev.Key.SourceIndex = it.index
		ev.Source = it.name

		it.ev = ev
		it.count++
		t := ev.Time()
		if it.count == 1 {
			it.first = t
		}
		it.last = t
		return true
	}

	it.live = false
	it.ev = nil
	it.err = it.cur.Err()
	return false
}

func (m *MergeCursor) Next() bool {
	if m.err != nil || m.closed {
		m.cur = nil
		return false
	}

	// Advance the input whose head we handed out last time. Doing it here
	// rather than eagerly is what keeps Event() valid until the next Next call.
	if m.pending != nil {
		it := m.pending
		m.pending = nil

		if m.advance(it) {
			heap.Fix(&m.h, m.indexInHeap(it))
		} else {
			heap.Remove(&m.h, m.indexInHeap(it))

			if it.err != nil {
				srcErr := &SourceError{Source: it.name, Index: it.index, Err: it.err}
				if m.policy == FailFast {
					m.err = srcErr
					m.closeAll()
					m.cur = nil
					return false
				}
				m.logger.WithError(srcErr).Warnf("dropping market data source %s", it.name)
			}

			// An exhausted input is closed eagerly. This is what bounds file
			// descriptor use when a CSV source spans hundreds of daily files.
			m.closeItem(it)
		}
	}

	if len(m.h) == 0 {
		m.cur = nil
		return false
	}

	top := m.h[0]
	m.cur = top.ev
	m.pending = top

	if m.hasLast && m.cur.Key.Compare(m.last) < 0 {
		m.err = fmt.Errorf("%w: merge emitted %+v after %+v", ErrOutOfOrder, m.cur.Key, m.last)
		m.closeAll()
		m.cur = nil
		return false
	}
	m.last, m.hasLast = m.cur.Key, true

	return true
}

// indexInHeap finds it in the heap. The heap holds at most one entry per
// source, and k is the number of configured sources — a handful in practice —
// so the linear scan is cheaper than maintaining a position field through
// every Swap.
func (m *MergeCursor) indexInHeap(it *mergeItem) int {
	for i, x := range m.h {
		if x == it {
			return i
		}
	}
	return -1
}

func (m *MergeCursor) Event() *Event { return m.cur }
func (m *MergeCursor) Err() error    { return m.err }

// Stats reports what each input contributed. Call it after the merge is
// exhausted; it is also logged by Close.
func (m *MergeCursor) Stats() []SourceStats {
	out := make([]SourceStats, len(m.items))
	for i, it := range m.items {
		out[i] = SourceStats{
			Name:  it.name,
			Index: it.index,
			Count: it.count,
			First: it.first,
			Last:  it.last,
			Err:   it.err,
		}
	}
	return out
}

// closeItem releases one input's cursor exactly once, logging rather than
// propagating a close error on the eager path.
func (m *MergeCursor) closeItem(it *mergeItem) error {
	if it.closed {
		return nil
	}
	it.closed = true
	it.live = false

	err := it.cur.Close()
	if err != nil {
		m.logger.WithError(err).Warnf("closing market data source %s", it.name)
	}
	return err
}

// closeAll closes every input that is still open, including ones already
// exhausted or failed. An errored cursor stops being live before it is closed,
// so this must key off closed rather than live.
func (m *MergeCursor) closeAll() error {
	var err error
	for _, it := range m.items {
		err = multierr.Append(err, m.closeItem(it))
	}
	return err
}

// Close closes every input exactly once and logs per-source statistics.
func (m *MergeCursor) Close() error {
	if m.closed {
		return nil
	}
	m.closed = true
	m.cur = nil
	m.pending = nil

	err := m.closeAll()

	for _, s := range m.Stats() {
		if s.Count == 0 {
			m.logger.Warnf("market data source %s produced no events", s.Name)
			continue
		}
		m.logger.Infof("market data source %s: %d events, %s .. %s",
			s.Name, s.Count, s.First.Format(time.RFC3339Nano), s.Last.Format(time.RFC3339Nano))
	}

	return err
}

// CheckCoverage reports whether the sources together serve every subscription
// in req, naming what is missing and which source would have to provide it.
func CheckCoverage(sources []Source, req Request) error {
	var missing []types.Subscription

	for _, sub := range req.Subscriptions {
		served := false
		for _, src := range sources {
			if src.Capabilities().Supports(sub) {
				served = true
				break
			}
		}
		if !served {
			missing = append(missing, sub)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	names := make([]string, len(sources))
	for i, src := range sources {
		names[i] = src.Name()
	}

	var sb strings.Builder
	sb.WriteString("marketdata: no configured source serves ")
	for i, sub := range missing {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%s %s", sub.Symbol, sub.Channel)
		if len(sub.Options.Interval) > 0 {
			fmt.Fprintf(&sb, " (%s)", sub.Options.Interval)
		}
	}
	fmt.Fprintf(&sb, "; configured sources are %v", names)

	return errors.New(sb.String())
}

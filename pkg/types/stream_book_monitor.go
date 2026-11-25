package types

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var streamBookStaleMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_stream_order_book_stale",
		Help: "whether a stream order book's update frequency is too low (stale): 1 = stale, 0 = fresh",
	}, []string{"symbol", "exchange"})

func init() {
	prometheus.MustRegister(streamBookStaleMetrics)
}

// StreamBookMonitor tracks whether a StreamOrderBook is still receiving
// update/snapshot events from its websocket stream often enough. When the
// time since the last event exceeds StaleThreshold, IsStale reports true so
// the caller can decide to call PollUpdate explicitly. This type never polls
// on its own.
type StreamBookMonitor struct {
	// Enabled enables the staleness tracking and polling fallback.
	Enabled bool `json:"enabled"`

	// StaleThreshold is the maximum acceptable time since the last order book
	// update/snapshot event before the book is considered stale.
	StaleThreshold Duration `json:"staleThreshold"`

	book *StreamOrderBook

	mu            sync.Mutex
	lastEventTime time.Time

	// staleMetric mirrors bbgo_connection_status: 1 = stale, 0 = fresh.
	staleMetric prometheus.Gauge
}

func (m *StreamBookMonitor) Defaults() error {
	if m.StaleThreshold == 0 {
		m.StaleThreshold = Duration(5 * time.Second)
	}
	return nil
}

func (m *StreamBookMonitor) Validate() error {
	if m.StaleThreshold <= 0 {
		return fmt.Errorf("streamBookMonitor.staleThreshold must be positive")
	}
	return nil
}

// Bind attaches the monitor to book's OnUpdate/OnSnapshot callbacks and binds
// the Prometheus metric using the book's symbol/exchange labels. Call after
// book.BindStream.
func (m *StreamBookMonitor) Bind(book *StreamOrderBook) {
	m.book = book
	m.staleMetric = streamBookStaleMetrics.With(prometheus.Labels{
		"symbol":   book.Symbol,
		"exchange": string(book.Exchange),
	})

	book.OnUpdate(func(update SliceOrderBook) { m.recordEvent(update) })
	book.OnSnapshot(func(snapshot SliceOrderBook) { m.recordEvent(snapshot) })
}

// recordEvent updates lastEventTime to book.Time (falls back to time.Now if
// the exchange doesn't report one), but only if book.IsValid() — an
// update/snapshot with an empty side or a crossed bid/ask does not advance
// lastEventTime. Either way, the stale metric is refreshed from the
// resulting staleness state, via the same check IsStale uses. Deliberately
// NOT called from PollUpdate — a REST poll must not mask stream degradation
// by making the book look fresh again.
func (m *StreamBookMonitor) recordEvent(book SliceOrderBook) {
	if ok, _ := book.IsValid(); ok {
		m.mu.Lock()
		m.lastEventTime = defaultTime(book.Time, time.Now)
		m.mu.Unlock()
	}

	if m.isStale(time.Now()) {
		m.staleMetric.Set(1)
	} else {
		m.staleMetric.Set(0)
	}
}

// isStale reports whether lastEventTime is zero (no event observed yet) or
// now has moved more than StaleThreshold past it. Pure state read, with no
// metric side effect — IsStale and recordEvent both use it and handle
// updating the stale metric themselves.
func (m *StreamBookMonitor) isStale(now time.Time) bool {
	m.mu.Lock()
	lastEventTime := m.lastEventTime
	m.mu.Unlock()

	return lastEventTime.IsZero() || now.Sub(lastEventTime) > m.StaleThreshold.Duration()
}

// IsStale reports whether the book hasn't received an update/snapshot event
// for longer than StaleThreshold as of now, and updates the stale metric
// accordingly. Returns true before any event has been observed: a book that
// hasn't received even its first snapshot is stale by definition.
func (m *StreamBookMonitor) IsStale(now time.Time) bool {
	stale := m.isStale(now)
	if stale {
		m.staleMetric.Set(1)
	} else {
		m.staleMetric.Set(0)
	}

	return stale
}

// PollUpdate explicitly fetches a fresh order book snapshot via REST and
// loads it into the bound book. It must be invoked explicitly by the caller;
// this type never calls it on its own.
func (m *StreamBookMonitor) PollUpdate(ctx context.Context, query ExchangeDepthQueryService) error {
	snapshot, _, err := query.QueryDepth(ctx, m.book.Symbol)
	if err != nil {
		return err
	}

	m.book.Load(snapshot)
	m.book.UpdateMetrics(snapshot.Time)

	logger.Infof("polled order book snapshot for %s via REST due to low update frequency", m.book.Symbol)

	return nil
}

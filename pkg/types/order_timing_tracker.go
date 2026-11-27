package types

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const (
	// DefaultOrderTTL is the default time-to-live for order timing records (3 minutes).
	DefaultOrderTTL = 3 * time.Minute

	// DefaultOrderPruneInterval is the default interval for automatic background pruning.
	DefaultOrderPruneInterval = 30 * time.Second
)

// OrderTimingRecord tracks the lifecycle timestamps and order snapshots:
//  1. SendOrderAt: the time we're sending the order to the API.
//  2. ServerCreatedAt: the time the order is created on the server.
//  3. SendCancelAt: the time we're sending the cancel order request to the API.
//  4. ServerCancelledAt: the time the order is cancelled on the server.
//  5. ServerFilledAt: the time the order is (fully) filled on the server.
//  6. OrderBeforeCancel: snapshot of the order before sending the cancel request.
//  7. OrderAfterCancel: snapshot of the order after the cancel/fill event or status is
//     received from the server.
type OrderTimingRecord struct {
	OrderID           uint64    `json:"order_id"`
	SendOrderAt       time.Time `json:"send_order_at,omitzero"`
	ServerCreatedAt   time.Time `json:"server_created_at,omitzero"`
	SendCancelAt      time.Time `json:"send_cancel_at,omitzero"`
	ServerCancelledAt time.Time `json:"server_cancelled_at,omitzero"`
	ServerFilledAt    time.Time `json:"server_filled_at,omitzero"`
	OrderBeforeCancel *Order    `json:"order_before_cancel,omitzero"`
	OrderAfterCancel  *Order    `json:"order_after_cancel,omitzero"`
	UpdatedAt         time.Time `json:"updated_at,omitzero"`

	// ackReported and cancelReported keep each latency out of the metrics more than once.
	// A cancelled order can reach RecordServerCancelledOrder twice, once for the cancelled
	// status and once for the terminal update that follows it.
	ackReported    bool
	cancelReported bool
}

// clone creates a deep copy of OrderTimingRecord.
func (r *OrderTimingRecord) clone() OrderTimingRecord {
	if r == nil {
		return OrderTimingRecord{}
	}
	cp := *r
	if r.OrderBeforeCancel != nil {
		before := *r.OrderBeforeCancel
		cp.OrderBeforeCancel = &before
	}
	if r.OrderAfterCancel != nil {
		after := *r.OrderAfterCancel
		cp.OrderAfterCancel = &after
	}
	return cp
}

// CancelOrderAt returns SendCancelAt for backward compatibility.
func (r *OrderTimingRecord) CancelOrderAt() time.Time {
	if r == nil {
		return time.Time{}
	}
	return r.SendCancelAt
}

// LatestTime returns the latest recorded timestamp for this order.
func (r *OrderTimingRecord) LatestTime() time.Time {
	if r == nil {
		return time.Time{}
	}
	latest := r.UpdatedAt
	if r.SendOrderAt.After(latest) {
		latest = r.SendOrderAt
	}
	if r.ServerCreatedAt.After(latest) {
		latest = r.ServerCreatedAt
	}
	if r.SendCancelAt.After(latest) {
		latest = r.SendCancelAt
	}
	if r.ServerCancelledAt.After(latest) {
		latest = r.ServerCancelledAt
	}
	if r.ServerFilledAt.After(latest) {
		latest = r.ServerFilledAt
	}
	return latest
}

// IsExpired checks if the record has expired based on its latest timestamp and the given TTL.
func (r *OrderTimingRecord) IsExpired(now time.Time, ttl time.Duration) bool {
	if r == nil {
		return true
	}
	latest := r.LatestTime()
	if latest.IsZero() {
		return true
	}
	return now.Sub(latest) > ttl
}

// LatencyToServer calculates the duration between sending the order and when the server created it.
// Returns 0 if either timestamp is missing.
func (r *OrderTimingRecord) LatencyToServer() time.Duration {
	if r == nil || r.SendOrderAt.IsZero() || r.ServerCreatedAt.IsZero() {
		return 0
	}
	return r.ServerCreatedAt.Sub(r.SendOrderAt)
}

// LatencyToCancel calculates the duration between sending the cancel request and when the server cancelled it.
// Returns 0 if either timestamp is missing.
func (r *OrderTimingRecord) LatencyToCancel() time.Duration {
	if r == nil || r.SendCancelAt.IsZero() || r.ServerCancelledAt.IsZero() {
		return 0
	}
	return r.ServerCancelledAt.Sub(r.SendCancelAt)
}

// AdditionalFilledVolume returns the additional volume filled between when the cancel request
// was sent and when the cancel/terminal event was received from the server.
// Returns zero if snapshots are unavailable or if no additional volume was filled.
func (r *OrderTimingRecord) AdditionalFilledVolume() fixedpoint.Value {
	if r == nil || r.OrderBeforeCancel == nil || r.OrderAfterCancel == nil {
		return fixedpoint.Zero
	}

	before := r.OrderBeforeCancel.GetRemainingQuantity()
	after := r.OrderAfterCancel.GetRemainingQuantity()
	if before.Compare(after) > 0 {
		return before.Sub(after)
	}
	return fixedpoint.Zero
}

// HasAdditionalFillAfterCancel returns true if additional volume was filled after sending cancel.
func (r *OrderTimingRecord) HasAdditionalFillAfterCancel() bool {
	return r.AdditionalFilledVolume().Sign() > 0
}

// CancelWorked checks if the cancel operation was successful.
// It returns true if the order was cancelled on the server and no additional volume was filled
// after sending the cancel request.
// Returns false if the order was filled/partially filled more after sending cancel,
// or if the final server status is not Canceled.
func (r *OrderTimingRecord) CancelWorked() bool {
	if r == nil || r.OrderAfterCancel == nil {
		return false
	}
	if r.OrderAfterCancel.Status != OrderStatusCanceled {
		return false
	}
	if r.OrderBeforeCancel != nil &&
		r.OrderAfterCancel.GetRemainingQuantity().Compare(r.OrderBeforeCancel.GetRemainingQuantity()) < 0 {
		return false
	}
	return true
}

// IsCancelSuccessful is an alias for CancelWorked.
func (r *OrderTimingRecord) IsCancelSuccessful() bool {
	return r.CancelWorked()
}

// Format returns a formatted string representation of the order timing record.
func (r *OrderTimingRecord) Format() string {
	if r == nil {
		return "<nil>"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "OrderTimingRecord[ID=%d", r.OrderID)
	if !r.SendOrderAt.IsZero() {
		fmt.Fprintf(&sb, ", SendOrderAt=%s", r.SendOrderAt.Format(time.StampMilli))
	}
	if !r.ServerCreatedAt.IsZero() {
		fmt.Fprintf(&sb, ", ServerCreatedAt=%s", r.ServerCreatedAt.Format(time.StampMilli))
	}
	if latency := r.LatencyToServer(); latency > 0 {
		fmt.Fprintf(&sb, " (latency=%s)", latency)
	}
	if !r.SendCancelAt.IsZero() {
		fmt.Fprintf(&sb, ", SendCancelAt=%s", r.SendCancelAt.Format(time.StampMilli))
	}
	if !r.ServerCancelledAt.IsZero() {
		fmt.Fprintf(&sb, ", ServerCancelledAt=%s", r.ServerCancelledAt.Format(time.StampMilli))
	}
	if cancelLatency := r.LatencyToCancel(); cancelLatency > 0 {
		fmt.Fprintf(&sb, " (cancelLatency=%s)", cancelLatency)
	}
	if !r.ServerFilledAt.IsZero() {
		fmt.Fprintf(&sb, ", ServerFilledAt=%s", r.ServerFilledAt.Format(time.StampMilli))
	}
	if r.OrderBeforeCancel != nil {
		fmt.Fprintf(&sb, ", BeforeCancelRemain=%s", r.OrderBeforeCancel.GetRemainingQuantity().String())
	}
	if r.OrderAfterCancel != nil {
		fmt.Fprintf(&sb, ", AfterCancelRemain=%s, CancelStatus=%v",
			r.OrderAfterCancel.GetRemainingQuantity().String(), r.OrderAfterCancel.Status)
		if r.OrderBeforeCancel != nil {
			fmt.Fprintf(&sb, ", AdditionalFilled=%s", r.AdditionalFilledVolume().String())
		}
		fmt.Fprintf(&sb, ", CancelWorked=%t", r.CancelWorked())
	}
	sb.WriteString("]")
	return sb.String()
}

// ClockOffsetEstimator estimates the offset between the exchange clock and the local clock,
// and collects the samples it needs from the timestamps passed to it.
type ClockOffsetEstimator interface {
	// Observe records one pairing of an exchange timestamp with the local time it arrived.
	Observe(serverTime, localTime time.Time)

	// Offset returns the estimated exchangeClock - localClock, and whether an estimate is
	// available at all.
	Offset() (time.Duration, bool)
}

// OrderTimingTrackerOption allows customizing OrderTimingTracker configuration.
type OrderTimingTrackerOption func(*OrderTimingTracker)

// OttWithSymbol sets the symbol the tracker reports its metrics under. Without it the
// tracker still collects timings but reports no metrics, since there is no symbol label to
// use.
func OttWithSymbol(symbol string) OrderTimingTrackerOption {
	return func(s *OrderTimingTracker) {
		s.symbol = symbol
	}
}

// OttWithClockOffsetEstimator sets the estimator used to correct the latencies that run
// from a local timestamp to an exchange timestamp. Without it only the uncorrected
// latencies are reported.
func OttWithClockOffsetEstimator(estimator ClockOffsetEstimator) OrderTimingTrackerOption {
	return func(s *OrderTimingTracker) {
		s.clock = estimator
	}
}

// OttWithTTL sets a custom time-to-live for orders in the tracker.
func OttWithTTL(ttl time.Duration) OrderTimingTrackerOption {
	return func(s *OrderTimingTracker) {
		if ttl > 0 {
			s.ttl = ttl
		}
	}
}

// OttWithLogger sets the logger used to log pruned records. Without it, pruning is silent.
func OttWithLogger(logger logrus.FieldLogger) OrderTimingTrackerOption {
	return func(s *OrderTimingTracker) {
		s.logger = logger
	}
}

// OttWithPruneInterval sets a custom interval for background pruning.
func OttWithPruneInterval(interval time.Duration) OrderTimingTrackerOption {
	return func(s *OrderTimingTracker) {
		if interval > 0 {
			s.pruneInterval = interval
		}
	}
}

// OrderTimingTracker maintains a thread-safe collection of order timing records with automatic pruning of expired entries.
type OrderTimingTracker struct {
	mu            sync.RWMutex
	orders        map[uint64]*OrderTimingRecord
	ttl           time.Duration
	pruneInterval time.Duration

	logger logrus.FieldLogger

	// symbol labels the reported metrics, empty disables reporting them.
	symbol string

	// clock corrects the latencies that span the local and the exchange clock, nil reports
	// only the uncorrected ones.
	clock ClockOffsetEstimator

	cancelMu      sync.Mutex
	cancel        context.CancelFunc
	stopCh        chan struct{}
	pruningActive atomic.Bool
}

// NewOrderTimingTracker creates a new OrderTimingTracker with default or custom options.
func NewOrderTimingTracker(opts ...OrderTimingTrackerOption) *OrderTimingTracker {
	s := &OrderTimingTracker{
		orders:        make(map[uint64]*OrderTimingRecord),
		ttl:           DefaultOrderTTL,
		pruneInterval: DefaultOrderPruneInterval,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// SetClockOffsetEstimator sets the clock offset estimator after construction, for the wiring
// that only has the estimator once every market is already built.
func (s *OrderTimingTracker) SetClockOffsetEstimator(estimator ClockOffsetEstimator) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.clock = estimator
}

// TTL returns the configured TTL for the tracker.
func (s *OrderTimingTracker) TTL() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ttl
}

// getOrCreate returns an existing record or creates a new one under lock. Must be called with s.mu held.
func (s *OrderTimingTracker) getOrCreate(orderID uint64) *OrderTimingRecord {
	if s.orders == nil {
		s.orders = make(map[uint64]*OrderTimingRecord)
	}
	record, ok := s.orders[orderID]
	if !ok {
		record = &OrderTimingRecord{
			OrderID: orderID,
		}
		s.orders[orderID] = record
		s.reportSize()
	}
	return record
}

// RecordSendOrder records the time the order was sent to the API.
func (s *OrderTimingTracker) RecordSendOrder(orderID uint64, sendAt time.Time) {
	if orderID == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record := s.getOrCreate(orderID)
	record.SendOrderAt = sendAt
	record.UpdatedAt = time.Now()
	s.reportAckLatency(record)
}

// RecordServerCreated records the time the order was created on the server.
func (s *OrderTimingTracker) RecordServerCreated(orderID uint64, createdAt time.Time) {
	if orderID == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	record := s.getOrCreate(orderID)
	record.ServerCreatedAt = createdAt
	record.UpdatedAt = now

	s.observeClockSample(createdAt, now)
	s.reportAckLatency(record)
}

// RecordSendCancel records the time the cancel order request was sent to the API.
func (s *OrderTimingTracker) RecordSendCancel(orderID uint64, sendAt time.Time) {
	if orderID == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record := s.getOrCreate(orderID)
	record.SendCancelAt = sendAt
	record.UpdatedAt = time.Now()
	s.reportCancelOutcome(record)
}

// RecordSendCancelOrder records the cancel order request time and stores the snapshot of the order before cancel.
func (s *OrderTimingTracker) RecordSendCancelOrder(order Order, sendAt time.Time) {
	if order.OrderID == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record := s.getOrCreate(order.OrderID)
	record.SendCancelAt = sendAt
	orderCopy := order
	record.OrderBeforeCancel = &orderCopy
	record.UpdatedAt = time.Now()
	s.reportCancelOutcome(record)
}

// RecordCancelOrder is an alias for RecordSendCancel for backward compatibility.
func (s *OrderTimingTracker) RecordCancelOrder(orderID uint64, cancelAt time.Time) {
	s.RecordSendCancel(orderID, cancelAt)
}

// RecordServerCancelled records the time the order was cancelled on the server.
func (s *OrderTimingTracker) RecordServerCancelled(orderID uint64, cancelledAt time.Time) {
	if orderID == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	record := s.getOrCreate(orderID)
	record.ServerCancelledAt = cancelledAt
	record.UpdatedAt = now

	s.observeClockSample(cancelledAt, now)
	s.reportCancelOutcome(record)
}

// RecordServerCancelledOrder records the cancellation event/status and stores the snapshot of the order after cancel.
func (s *OrderTimingTracker) RecordServerCancelledOrder(order Order, cancelledAt time.Time) {
	if order.OrderID == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	record := s.getOrCreate(order.OrderID)
	record.ServerCancelledAt = cancelledAt
	orderCopy := order
	record.OrderAfterCancel = &orderCopy
	record.UpdatedAt = now

	s.observeClockSample(cancelledAt, now)
	s.reportCancelOutcome(record)
}

// RecordServerFilledOrder records the time the order was (fully) filled on the server and
// stores the final order snapshot.
func (s *OrderTimingTracker) RecordServerFilledOrder(order Order, filledAt time.Time) {
	if order.OrderID == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	record := s.getOrCreate(order.OrderID)
	record.ServerFilledAt = filledAt
	orderCopy := order
	record.OrderAfterCancel = &orderCopy
	record.UpdatedAt = now

	s.observeClockSample(filledAt, now)
}

// RecordOrderUpdate records the latest order update snapshot into the tracker.
func (s *OrderTimingTracker) RecordOrderUpdate(order Order) {
	if order.OrderID == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	record := s.getOrCreate(order.OrderID)
	orderCopy := order
	record.OrderAfterCancel = &orderCopy
	record.UpdatedAt = now

	// Every order update carries the exchange timestamp of the update, which is a clock
	// offset sample whether or not this order was ever cancelled.
	s.observeClockSample(order.UpdateTime.Time(), now)
}

// RecordOrder records both the send time and the server creation time (if non-zero) for an Order.
func (s *OrderTimingTracker) RecordOrder(order Order, sendOrderAt time.Time) {
	if order.OrderID == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	record := s.getOrCreate(order.OrderID)
	if !sendOrderAt.IsZero() {
		record.SendOrderAt = sendOrderAt
	}

	createdAt := order.CreationTime.Time()
	if hasExchangeTimestamp(createdAt) {
		record.ServerCreatedAt = createdAt
		s.observeClockSample(createdAt, now)
	}
	record.UpdatedAt = now

	s.reportAckLatency(record)
}

// Set adds or replaces an entire OrderTimingRecord in the tracker.
func (s *OrderTimingTracker) Set(record OrderTimingRecord) {
	if record.OrderID == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.orders == nil {
		s.orders = make(map[uint64]*OrderTimingRecord)
	}

	rec := record.clone()
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = time.Now()
	}
	s.orders[record.OrderID] = &rec
	s.reportSize()
}

// Get retrieves a copy of the OrderTimingRecord for the given order ID.
func (s *OrderTimingTracker) Get(orderID uint64) (OrderTimingRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.orders[orderID]
	if !ok || record == nil {
		return OrderTimingRecord{}, false
	}
	return record.clone(), true
}

// Remove removes the timing record for the specified order ID.
func (s *OrderTimingTracker) Remove(orderID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.orders != nil {
		delete(s.orders, orderID)
	}
	s.reportSize()
}

// Clear removes all records from the tracker.
func (s *OrderTimingTracker) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	clear(s.orders)
	s.reportSize()
}

// Count returns the number of records currently stored.
func (s *OrderTimingTracker) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.orders)
}

// Each executes fn for each record in the tracker.
func (s *OrderTimingTracker) Each(fn func(record OrderTimingRecord)) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, record := range s.orders {
		if record != nil {
			fn(record.clone())
		}
	}
}

// All returns a slice containing copies of all records in the tracker.
func (s *OrderTimingTracker) All() []OrderTimingRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]OrderTimingRecord, 0, len(s.orders))
	for _, record := range s.orders {
		if record != nil {
			records = append(records, record.clone())
		}
	}
	return records
}

// Prune removes all records that have expired based on the configured TTL.
// Returns the number of pruned records.
func (s *OrderTimingTracker) Prune() int {
	return s.PruneOlderThan(s.ttl)
}

// PruneOlderThan removes records whose latest timestamp is older than the given TTL.
// Returns the number of pruned records.
func (s *OrderTimingTracker) PruneOlderThan(ttl time.Duration) int {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	prunedCount := 0
	for id, record := range s.orders {
		if record == nil || record.IsExpired(now, ttl) {
			s.pruneRecord(id, record)
			prunedCount++
		}
	}
	s.reportSize()

	return prunedCount
}

// PruneBefore removes records whose latest timestamp is before the given cutoff time.
// Returns the number of pruned records.
func (s *OrderTimingTracker) PruneBefore(cutoff time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	prunedCount := 0
	for id, record := range s.orders {
		if record == nil || record.LatestTime().Before(cutoff) {
			s.pruneRecord(id, record)
			prunedCount++
		}
	}
	s.reportSize()

	return prunedCount
}

func (s *OrderTimingTracker) pruneRecord(id uint64, record *OrderTimingRecord) {
	if s.logger != nil {
		s.logger.Debugf("pruning record: %s", record.Format())
	}
	delete(s.orders, id)
}

// StartPruning starts a background goroutine that periodically prunes expired records.
// If interval <= 0, the tracker's configured pruneInterval is used.
func (s *OrderTimingTracker) StartPruning(ctx context.Context, interval time.Duration) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()

	if s.pruningActive.Load() {
		return
	}

	if interval <= 0 {
		interval = s.pruneInterval
	}

	pruneCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.stopCh = make(chan struct{})
	s.pruningActive.Store(true)

	go func() {
		defer func() {
			s.pruningActive.Store(false)
			close(s.stopCh)
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-pruneCtx.Done():
				return
			case <-ticker.C:
				s.Prune()
			}
		}
	}()
}

// StopPruning stops the background pruning goroutine if it is active.
func (s *OrderTimingTracker) StopPruning() {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()

	if !s.pruningActive.Load() {
		return
	}

	if s.cancel != nil {
		s.cancel()
	}

	if s.stopCh != nil {
		<-s.stopCh
	}
}

// IsPruningActive returns true if background pruning is currently running.
func (s *OrderTimingTracker) IsPruningActive() bool {
	return s.pruningActive.Load()
}

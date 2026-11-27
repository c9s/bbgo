package types

import (
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func timingTestOrder(id uint64, remainQuantity fixedpoint.Value, status OrderStatus) Order {
	quantity := fixedpoint.NewFromInt(1000)
	return Order{
		SubmitOrder: SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     SideTypeBuy,
			Price:    fixedpoint.NewFromInt(50000),
			Quantity: quantity,
		},
		OrderID:          id,
		Status:           status,
		ExecutedQuantity: quantity.Sub(remainQuantity),
	}
}

func TestOrderTimingTracker_BasicTracking(t *testing.T) {
	tracker := NewOrderTimingTracker()

	t1 := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 21, 10, 0, 0, 50*1000000, time.UTC) // +50ms
	t3 := time.Date(2026, 8, 21, 10, 0, 5, 0, time.UTC)          // +5s (send cancel)
	t4 := time.Date(2026, 8, 21, 10, 0, 5, 80*1000000, time.UTC) // +5s80ms (server cancelled)

	orderID := uint64(12345)

	t.Run("record send order", func(t *testing.T) {
		tracker.RecordSendOrder(orderID, t1)

		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		assert.Equal(t, orderID, rec.OrderID)
		assert.Equal(t, t1, rec.SendOrderAt)
		assert.True(t, rec.ServerCreatedAt.IsZero())
		assert.True(t, rec.SendCancelAt.IsZero())
		assert.True(t, rec.ServerCancelledAt.IsZero())
		assert.False(t, rec.UpdatedAt.IsZero())
	})

	t.Run("record server created", func(t *testing.T) {
		tracker.RecordServerCreated(orderID, t2)

		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		assert.Equal(t, t1, rec.SendOrderAt)
		assert.Equal(t, t2, rec.ServerCreatedAt)
		assert.Equal(t, 50*time.Millisecond, rec.LatencyToServer())
		assert.True(t, rec.SendCancelAt.IsZero())
		assert.True(t, rec.ServerCancelledAt.IsZero())
	})

	t.Run("record send cancel", func(t *testing.T) {
		tracker.RecordSendCancel(orderID, t3)

		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		assert.Equal(t, t1, rec.SendOrderAt)
		assert.Equal(t, t2, rec.ServerCreatedAt)
		assert.Equal(t, t3, rec.SendCancelAt)
		assert.Equal(t, t3, rec.CancelOrderAt())
		assert.True(t, rec.ServerCancelledAt.IsZero())
		// LatencyToCancel should be 0 since ServerCancelledAt is not yet set
		assert.Equal(t, time.Duration(0), rec.LatencyToCancel())
	})

	t.Run("record server cancelled", func(t *testing.T) {
		tracker.RecordServerCancelled(orderID, t4)

		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		assert.Equal(t, t1, rec.SendOrderAt)
		assert.Equal(t, t2, rec.ServerCreatedAt)
		assert.Equal(t, t3, rec.SendCancelAt)
		assert.Equal(t, t4, rec.ServerCancelledAt)
		// LatencyToCancel = t4 - t3 = 80ms
		assert.Equal(t, 80*time.Millisecond, rec.LatencyToCancel())
	})

	t.Run("format string", func(t *testing.T) {
		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		formatted := rec.Format()
		assert.Contains(t, formatted, "OrderTimingRecord[ID=12345")
		assert.Contains(t, formatted, "SendOrderAt=")
		assert.Contains(t, formatted, "ServerCreatedAt=")
		assert.Contains(t, formatted, "SendCancelAt=")
		assert.Contains(t, formatted, "ServerCancelledAt=")
	})
}

func TestOrderTimingTracker_CancelSnapshotAndEvaluation(t *testing.T) {
	t.Run("successful cancel without additional fill", func(t *testing.T) {
		tracker := NewOrderTimingTracker()
		orderID := uint64(1001)
		sendCancelTime := time.Date(2026, 8, 21, 10, 0, 1, 0, time.UTC)
		serverCancelledTime := time.Date(2026, 8, 21, 10, 0, 1, 50*1000000, time.UTC)

		orderBefore := timingTestOrder(orderID, fixedpoint.NewFromInt(1000), OrderStatusNew)

		tracker.RecordSendCancelOrder(orderBefore, sendCancelTime)

		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		require.NotNil(t, rec.OrderBeforeCancel)
		assert.Equal(t, fixedpoint.NewFromInt(1000), rec.OrderBeforeCancel.GetRemainingQuantity())
		assert.Nil(t, rec.OrderAfterCancel)
		assert.False(t, rec.CancelWorked())

		orderAfter := timingTestOrder(orderID, fixedpoint.NewFromInt(1000), OrderStatusCanceled)
		orderAfter.UpdateTime = Time(serverCancelledTime)

		tracker.RecordServerCancelledOrder(orderAfter, serverCancelledTime)

		rec, ok = tracker.Get(orderID)
		require.True(t, ok)
		require.NotNil(t, rec.OrderAfterCancel)
		assert.Equal(t, fixedpoint.NewFromInt(1000), rec.OrderAfterCancel.GetRemainingQuantity())
		assert.Equal(t, OrderStatusCanceled, rec.OrderAfterCancel.Status)
		assert.True(t, rec.AdditionalFilledVolume().IsZero())
		assert.False(t, rec.HasAdditionalFillAfterCancel())
		assert.True(t, rec.CancelWorked())
		assert.True(t, rec.IsCancelSuccessful())
		assert.Contains(t, rec.Format(), "CancelWorked=true")
	})

	t.Run("cancel failed because partially filled more before cancel took effect", func(t *testing.T) {
		tracker := NewOrderTimingTracker()
		orderID := uint64(1002)
		sendCancelTime := time.Date(2026, 8, 21, 10, 0, 2, 0, time.UTC)
		serverCancelledTime := time.Date(2026, 8, 21, 10, 0, 2, 60*1000000, time.UTC)

		// Order had 800 remaining quantity before we sent cancel
		orderBefore := timingTestOrder(orderID, fixedpoint.NewFromInt(800), OrderStatusNew)

		tracker.RecordSendCancelOrder(orderBefore, sendCancelTime)

		// Order ended up with only 300 remaining when server cancelled it (500 filled during in-flight cancel)
		orderAfter := timingTestOrder(orderID, fixedpoint.NewFromInt(300), OrderStatusCanceled)
		orderAfter.UpdateTime = Time(serverCancelledTime)

		tracker.RecordServerCancelledOrder(orderAfter, serverCancelledTime)

		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		assert.Equal(t, fixedpoint.NewFromInt(500), rec.AdditionalFilledVolume())
		assert.True(t, rec.HasAdditionalFillAfterCancel())
		assert.False(t, rec.CancelWorked())
		assert.False(t, rec.IsCancelSuccessful())
		assert.Contains(t, rec.Format(), "CancelWorked=false")
	})

	t.Run("cancel failed because order was completely filled", func(t *testing.T) {
		tracker := NewOrderTimingTracker()
		orderID := uint64(1003)
		sendCancelTime := time.Date(2026, 8, 21, 10, 0, 3, 0, time.UTC)
		terminalTime := time.Date(2026, 8, 21, 10, 0, 3, 40*1000000, time.UTC)

		orderBefore := timingTestOrder(orderID, fixedpoint.NewFromInt(1000), OrderStatusNew)

		tracker.RecordSendCancelOrder(orderBefore, sendCancelTime)

		// Server completely filled the order
		orderAfter := timingTestOrder(orderID, fixedpoint.Zero, OrderStatusFilled)
		orderAfter.UpdateTime = Time(terminalTime)

		tracker.RecordServerCancelledOrder(orderAfter, terminalTime)

		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		assert.Equal(t, fixedpoint.NewFromInt(1000), rec.AdditionalFilledVolume())
		assert.True(t, rec.HasAdditionalFillAfterCancel())
		assert.False(t, rec.CancelWorked())
		assert.False(t, rec.IsCancelSuccessful())
	})

	t.Run("snapshot immutability via clone", func(t *testing.T) {
		tracker := NewOrderTimingTracker()
		orderID := uint64(1004)
		sendCancelTime := time.Now()

		orderBefore := timingTestOrder(orderID, fixedpoint.NewFromInt(500), OrderStatusNew)

		tracker.RecordSendCancelOrder(orderBefore, sendCancelTime)

		// Modify caller's local copy
		orderBefore.ExecutedQuantity = orderBefore.Quantity

		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		assert.Equal(t, fixedpoint.NewFromInt(500), rec.OrderBeforeCancel.GetRemainingQuantity())

		// Modify retrieved pointer
		rec.OrderBeforeCancel.ExecutedQuantity = rec.OrderBeforeCancel.Quantity

		rec2, ok := tracker.Get(orderID)
		require.True(t, ok)
		assert.Equal(t, fixedpoint.NewFromInt(500), rec2.OrderBeforeCancel.GetRemainingQuantity())
	})

	t.Run("record order update latest snapshot", func(t *testing.T) {
		tracker := NewOrderTimingTracker()
		orderID := uint64(1005)

		orderUpdate := timingTestOrder(orderID, fixedpoint.NewFromInt(700), OrderStatusPartiallyFilled)

		tracker.RecordOrderUpdate(orderUpdate)

		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		require.NotNil(t, rec.OrderAfterCancel)
		assert.Equal(t, fixedpoint.NewFromInt(700), rec.OrderAfterCancel.GetRemainingQuantity())
		assert.Equal(t, OrderStatusPartiallyFilled, rec.OrderAfterCancel.Status)
	})

	t.Run("record server filled order", func(t *testing.T) {
		tracker := NewOrderTimingTracker()
		orderID := uint64(1006)
		filledAt := time.Date(2026, 8, 21, 10, 0, 6, 0, time.UTC)

		filledOrder := timingTestOrder(orderID, fixedpoint.Zero, OrderStatusFilled)

		tracker.RecordServerFilledOrder(filledOrder, filledAt)

		rec, ok := tracker.Get(orderID)
		require.True(t, ok)
		assert.Equal(t, filledAt, rec.ServerFilledAt)
		require.NotNil(t, rec.OrderAfterCancel)
		assert.Equal(t, OrderStatusFilled, rec.OrderAfterCancel.Status)
		assert.Contains(t, rec.Format(), "ServerFilledAt=")
	})
}

func TestOrderTimingTracker_RecordOrder(t *testing.T) {
	tracker := NewOrderTimingTracker()

	sendTime := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	createdTime := time.Date(2026, 8, 21, 12, 0, 0, 20*1000000, time.UTC)

	order := Order{
		OrderID:      888,
		CreationTime: Time(createdTime),
	}

	tracker.RecordOrder(order, sendTime)

	rec, ok := tracker.Get(888)
	require.True(t, ok)
	assert.Equal(t, uint64(888), rec.OrderID)
	assert.Equal(t, sendTime, rec.SendOrderAt)
	assert.Equal(t, createdTime, rec.ServerCreatedAt)
	assert.Equal(t, 20*time.Millisecond, rec.LatencyToServer())
}

func TestOrderTimingTracker_CRUD(t *testing.T) {
	tracker := NewOrderTimingTracker()

	t.Run("set and get", func(t *testing.T) {
		rec := OrderTimingRecord{
			OrderID:         100,
			SendOrderAt:     time.Now().Add(-1 * time.Minute),
			ServerCreatedAt: time.Now().Add(-50 * time.Second),
		}
		tracker.Set(rec)

		got, ok := tracker.Get(100)
		require.True(t, ok)
		assert.Equal(t, uint64(100), got.OrderID)
		assert.Equal(t, 1, tracker.Count())
	})

	t.Run("each and all", func(t *testing.T) {
		tracker.RecordSendOrder(200, time.Now())

		assert.Equal(t, 2, tracker.Count())

		all := tracker.All()
		assert.Len(t, all, 2)

		var ids []uint64
		tracker.Each(func(r OrderTimingRecord) {
			ids = append(ids, r.OrderID)
		})
		assert.Len(t, ids, 2)
	})

	t.Run("remove", func(t *testing.T) {
		tracker.Remove(100)
		_, ok := tracker.Get(100)
		assert.False(t, ok)
		assert.Equal(t, 1, tracker.Count())
	})

	t.Run("clear", func(t *testing.T) {
		tracker.Clear()
		assert.Equal(t, 0, tracker.Count())
	})
}

func TestOrderTimingTracker_Pruning(t *testing.T) {
	tracker := NewOrderTimingTracker(OttWithTTL(3 * time.Minute))
	assert.Equal(t, 3*time.Minute, tracker.TTL())

	now := time.Now()

	// Order 1: updated 5 minutes ago (expired)
	tracker.Set(OrderTimingRecord{
		OrderID:         1,
		SendOrderAt:     now.Add(-5 * time.Minute),
		ServerCreatedAt: now.Add(-5 * time.Minute),
		UpdatedAt:       now.Add(-5 * time.Minute),
	})

	// Order 2: updated 4 minutes ago (expired)
	tracker.Set(OrderTimingRecord{
		OrderID:      2,
		SendOrderAt:  now.Add(-4 * time.Minute),
		SendCancelAt: now.Add(-4 * time.Minute),
		UpdatedAt:    now.Add(-4 * time.Minute),
	})

	// Order 3: updated 2 minutes ago (not expired)
	tracker.Set(OrderTimingRecord{
		OrderID:         3,
		SendOrderAt:     now.Add(-2 * time.Minute),
		ServerCreatedAt: now.Add(-2 * time.Minute),
		UpdatedAt:       now.Add(-2 * time.Minute),
	})

	// Order 4: updated 10 seconds ago (not expired)
	tracker.Set(OrderTimingRecord{
		OrderID:         4,
		SendOrderAt:     now.Add(-10 * time.Second),
		ServerCreatedAt: now.Add(-10 * time.Second),
		UpdatedAt:       now.Add(-10 * time.Second),
	})

	assert.Equal(t, 4, tracker.Count())

	// Prune orders older than 3 minutes
	pruned := tracker.Prune()
	assert.Equal(t, 2, pruned)
	assert.Equal(t, 2, tracker.Count())

	_, ok1 := tracker.Get(1)
	assert.False(t, ok1)
	_, ok2 := tracker.Get(2)
	assert.False(t, ok2)

	_, ok3 := tracker.Get(3)
	assert.True(t, ok3)
	_, ok4 := tracker.Get(4)
	assert.True(t, ok4)
}

func TestOrderTimingTracker_PruneOlderThan(t *testing.T) {
	tracker := NewOrderTimingTracker()

	now := time.Now()

	tracker.Set(OrderTimingRecord{
		OrderID:   1,
		UpdatedAt: now.Add(-10 * time.Second),
	})
	tracker.Set(OrderTimingRecord{
		OrderID:   2,
		UpdatedAt: now.Add(-2 * time.Second),
	})

	pruned := tracker.PruneOlderThan(5 * time.Second)
	assert.Equal(t, 1, pruned)
	assert.Equal(t, 1, tracker.Count())

	_, ok2 := tracker.Get(2)
	assert.True(t, ok2)
}

func TestOrderTimingTracker_WithLogger(t *testing.T) {
	logger, hook := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	tracker := NewOrderTimingTracker(OttWithLogger(logger))

	now := time.Now()
	tracker.Set(OrderTimingRecord{
		OrderID:   1,
		UpdatedAt: now.Add(-10 * time.Second),
	})

	pruned := tracker.PruneOlderThan(5 * time.Second)
	assert.Equal(t, 1, pruned)

	entry := hook.LastEntry()
	require.NotNil(t, entry)
	assert.Equal(t, logrus.DebugLevel, entry.Level)
	assert.Contains(t, entry.Message, "pruning record")
	assert.Contains(t, entry.Message, "OrderTimingRecord[ID=1")
}

func TestOrderTimingTracker_PruneBefore(t *testing.T) {
	tracker := NewOrderTimingTracker()

	t1 := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 21, 10, 2, 0, 0, time.UTC)
	t3 := time.Date(2026, 8, 21, 10, 5, 0, 0, time.UTC)

	tracker.Set(OrderTimingRecord{
		OrderID:     1,
		SendOrderAt: t1,
		UpdatedAt:   t1,
	})
	tracker.Set(OrderTimingRecord{
		OrderID:     2,
		SendOrderAt: t2,
		UpdatedAt:   t2,
	})
	tracker.Set(OrderTimingRecord{
		OrderID:     3,
		SendOrderAt: t3,
		UpdatedAt:   t3,
	})

	cutoff := time.Date(2026, 8, 21, 10, 3, 0, 0, time.UTC)
	pruned := tracker.PruneBefore(cutoff)
	assert.Equal(t, 2, pruned)
	assert.Equal(t, 1, tracker.Count())

	_, ok3 := tracker.Get(3)
	assert.True(t, ok3)
}

func TestOrderTimingTracker_BackgroundPruning(t *testing.T) {
	tracker := NewOrderTimingTracker(
		OttWithTTL(20*time.Millisecond),
		OttWithPruneInterval(10*time.Millisecond),
	)

	ctx := t.Context()
	tracker.StartPruning(ctx, 10*time.Millisecond)
	assert.True(t, tracker.IsPruningActive())

	now := time.Now()
	tracker.Set(OrderTimingRecord{
		OrderID:   10,
		UpdatedAt: now.Add(-100 * time.Millisecond),
	})
	tracker.Set(OrderTimingRecord{
		OrderID:   20,
		UpdatedAt: now,
	})

	assert.Eventually(t, func() bool {
		_, ok10 := tracker.Get(10)
		return !ok10
	}, 500*time.Millisecond, 20*time.Millisecond)

	tracker.StopPruning()
	assert.False(t, tracker.IsPruningActive())
}

func TestOrderTimingTracker_Concurrency(t *testing.T) {
	tracker := NewOrderTimingTracker(
		OttWithTTL(50*time.Millisecond),
		OttWithPruneInterval(20*time.Millisecond),
	)

	ctx := t.Context()
	tracker.StartPruning(ctx, 20*time.Millisecond)
	defer tracker.StopPruning()

	var wg sync.WaitGroup
	numWorkers := 10
	ordersPerWorker := 50

	for w := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := range ordersPerWorker {
				orderID := uint64(workerID*1000 + i + 1)
				sendTime := time.Now()
				tracker.RecordSendOrder(orderID, sendTime)
				tracker.RecordServerCreated(orderID, sendTime.Add(5*time.Millisecond))
				tracker.RecordCancelOrder(orderID, sendTime.Add(10*time.Millisecond))

				_, _ = tracker.Get(orderID)
				_ = tracker.Count()
			}
		}(w)
	}

	wg.Wait()
}

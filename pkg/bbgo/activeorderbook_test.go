package bbgo

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestActiveOrderBook_pendingOrders(t *testing.T) {
	now := time.Now()
	t1 := now
	t2 := now.Add(time.Millisecond)

	ob := NewActiveOrderBook("BTCUSDT")

	filled := false
	ob.OnFilled(func(o types.Order) {
		filled = true
	})

	quantity := Number("0.01")
	orderUpdate1 := types.Order{
		OrderID: 99,
		SubmitOrder: types.SubmitOrder{
			Symbol:       "BTCUSDT",
			Side:         types.SideTypeBuy,
			Type:         types.OrderTypeLimit,
			Quantity:     quantity,
			Price:        Number(19000.0),
			AveragePrice: fixedpoint.Zero,
			StopPrice:    fixedpoint.Zero,
		},
		ExecutedQuantity: Number(0.0),
		Status:           types.OrderStatusNew,
		CreationTime:     types.Time(t1),
		UpdateTime:       types.Time(t1),
	}

	orderUpdate2 := types.Order{
		OrderID: 99,
		SubmitOrder: types.SubmitOrder{
			Symbol:       "BTCUSDT",
			Side:         types.SideTypeBuy,
			Type:         types.OrderTypeLimit,
			Quantity:     quantity,
			Price:        Number(19000.0),
			AveragePrice: fixedpoint.Zero,
			StopPrice:    fixedpoint.Zero,
		},
		ExecutedQuantity: quantity,
		Status:           types.OrderStatusFilled,
		CreationTime:     types.Time(t1),
		UpdateTime:       types.Time(t2),
	}

	assert.True(t, isNewerOrderUpdate(orderUpdate2, orderUpdate1), "orderUpdate2 should be newer than orderUpdate1")

	// if we received filled order first
	// should be added to pending orders
	ob.Update(orderUpdate2)
	assert.Len(t, ob.pendingOrderUpdates.Orders(), 1)

	o99, ok := ob.pendingOrderUpdates.Get(99)
	if assert.True(t, ok) {
		assert.Equal(t, types.OrderStatusFilled, o99.Status)
	}

	// when adding the older order update to the book,
	// it should trigger the filled event once the order is registered to the active order book
	ob.Add(orderUpdate1)
	assert.True(t, filled, "filled event should be fired")
}

func TestActiveOrderBook_RestoreParametersOnUpdateHandler(t *testing.T) {
	now := time.Now()
	t1 := now
	t2 := now.Add(time.Millisecond)
	ob := NewActiveOrderBook("BTCUSDT")

	var updatedOrder types.Order
	ob.OnFilled(func(o types.Order) {
		updatedOrder = o
	})
	quantity := Number("0.01")
	orderUpdate1 := types.Order{
		OrderID: 99,
		SubmitOrder: types.SubmitOrder{
			Symbol:       "BTCUSDT",
			Side:         types.SideTypeBuy,
			Type:         types.OrderTypeLimit,
			Quantity:     quantity,
			Price:        Number(19000.0),
			AveragePrice: fixedpoint.Zero,
			StopPrice:    fixedpoint.Zero,
			Tag:          "tag1",
			GroupID:      uint32(1),
		},
		ExecutedQuantity: Number(0.0),
		Status:           types.OrderStatusNew,
		CreationTime:     types.Time(t1),
		UpdateTime:       types.Time(t1),
	}

	orderUpdate2 := types.Order{
		OrderID: 99,
		SubmitOrder: types.SubmitOrder{
			Symbol:       "BTCUSDT",
			Side:         types.SideTypeBuy,
			Type:         types.OrderTypeLimit,
			Quantity:     quantity,
			Price:        Number(19000.0),
			AveragePrice: fixedpoint.Zero,
			StopPrice:    fixedpoint.Zero,
		},
		ExecutedQuantity: quantity,
		Status:           types.OrderStatusFilled,
		CreationTime:     types.Time(t1),
		UpdateTime:       types.Time(t2),
	}
	ob.add(orderUpdate1)
	ob.orderUpdateHandler(orderUpdate2)
	assert.Equal(t, "tag1", updatedOrder.Tag)
	assert.Equal(t, uint32(1), updatedOrder.GroupID)

}

func Test_isNewerUpdate(t *testing.T) {
	a := types.Order{
		Status:           types.OrderStatusPartiallyFilled,
		ExecutedQuantity: number(0.2),
	}
	b := types.Order{
		Status:           types.OrderStatusPartiallyFilled,
		ExecutedQuantity: number(0.1),
	}
	ret := isNewerOrderUpdate(a, b)
	assert.True(t, ret)
}

func Test_isNewerUpdateTime(t *testing.T) {
	a := types.Order{
		UpdateTime: types.NewTimeFromUnix(200, 0),
	}
	b := types.Order{
		UpdateTime: types.NewTimeFromUnix(100, 0),
	}
	ret := isNewerOrderUpdateTime(a, b)
	assert.True(t, ret)
}

// TestActiveOrderBook_PendingOrderUpdatesMemoryLeak tests for memory leaks in pending order updates
// === RUN   TestActiveOrderBook_PendingOrderUpdatesMemoryLeak
// [Before] Alloc: 1921160 bytes (bytes of allocated heap objects)
// [Before] TotalAlloc: 3957240 bytes (cumulative bytes allocated for heap objects)
// [Before] Sys: 14304520 bytes (total bytes obtained from system)
// [Before] NumGC: 2 (number of completed GC cycles)
// [After] Alloc: 1878776 bytes (bytes of allocated heap objects)
// [After] TotalAlloc: 1556047376 bytes (cumulative bytes allocated for heap objects)
// [After] Sys: 19809544 bytes (total bytes obtained from system)
// [After] NumGC: 710 (number of completed GC cycles)
func TestActiveOrderBook_PendingOrderUpdatesMemoryLeak(t *testing.T) {
	book := NewActiveOrderBook("BTCUSDT")
	var memStatsBefore, memStatsAfter runtime.MemStats

	runtime.GC()

	runtime.ReadMemStats(&memStatsBefore)

	t.Logf("[Before] Alloc: %d bytes (bytes of allocated heap objects)", memStatsBefore.Alloc)
	t.Logf("[Before] TotalAlloc: %d bytes (cumulative bytes allocated for heap objects)", memStatsBefore.TotalAlloc)
	t.Logf("[Before] Sys: %d bytes (total bytes obtained from system)", memStatsBefore.Sys)
	t.Logf("[Before] NumGC: %d (number of completed GC cycles)", memStatsBefore.NumGC)

	orderCount := 1_000_000
	for i := 0; i < orderCount; i++ {
		order := types.Order{
			OrderID: uint64(100000 + i),
			SubmitOrder: types.SubmitOrder{
				Symbol: "BTCUSDT",
			},
			Status: types.OrderStatusNew,
		}
		book.pendingOrderUpdates.Add(order)

		// Move pending order updates to the main order store
		book.Add(order)

		// Simulate order being filled and removed
		book.Remove(order)
	}

	// force GC
	runtime.GC()
	runtime.ReadMemStats(&memStatsAfter)

	t.Logf("[After] Alloc: %d bytes (bytes of allocated heap objects)", memStatsAfter.Alloc)
	t.Logf("[After] TotalAlloc: %d bytes (cumulative bytes allocated for heap objects)", memStatsAfter.TotalAlloc)
	t.Logf("[After] Sys: %d bytes (total bytes obtained from system)", memStatsAfter.Sys)
	t.Logf("[After] NumGC: %d (number of completed GC cycles)", memStatsAfter.NumGC)

	assert.Less(t, memStatsAfter.Alloc, memStatsBefore.Alloc+uint64(orderCount/3)*1024, "memory allocation should not grow significantly")

	assert.Equal(t, 0, book.pendingOrderUpdates.Len(), "pendingOrderUpdates should be empty after cleanup")
	assert.Equal(t, 0, book.orders.Len(), "pendingOrderUpdates should be empty after cleanup")
}

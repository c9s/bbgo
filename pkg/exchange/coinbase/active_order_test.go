package coinbase

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestNewActiveOrderStore(t *testing.T) {
	store := newActiveOrderStore("test-key-1")
	assert.NotNil(t, store)
	assert.NotNil(t, store.orders)
	assert.Equal(t, 0, len(store.orders))

	store2 := newActiveOrderStore("test-key-1")
	assert.Equal(t, store, store2)

}

func TestActiveOrderStore_Add(t *testing.T) {
	store := newActiveOrderStore("test-key-2")

	submitOrder := types.SubmitOrder{
		Symbol:      "BTCUSD",
		Side:        types.SideTypeBuy,
		Type:        types.OrderTypeLimit,
		Quantity:    fixedpoint.NewFromFloat(0.1),
		Price:       fixedpoint.NewFromFloat(50000),
		Market:      types.Market{Symbol: "BTCUSD"},
		TimeInForce: types.TimeInForceGTC,
	}

	rawOrder := &api.CreateOrderResponse{
		ID:        "order-uuid-123",
		ProductID: "BTC-USD",
		Side:      api.SideTypeBuy,
		Type:      api.OrderTypeLimit,
		Price:     fixedpoint.NewFromFloat(50000),
		Size:      fixedpoint.NewFromFloat(0.1),
	}

	store.add(submitOrder, rawOrder)

	assert.Equal(t, 1, len(store.orders))
	activeOrder, ok := store.get("order-uuid-123")
	assert.True(t, ok)
	assert.NotNil(t, activeOrder)
	assert.Equal(t, submitOrder.Symbol, activeOrder.submitOrder.Symbol)
	assert.Equal(t, rawOrder.ID, activeOrder.rawOrder.ID)
}

func TestActiveOrderStore_GetByUUID(t *testing.T) {
	store := newActiveOrderStore("test-key-3")

	t.Run("existing order", func(t *testing.T) {
		submitOrder := types.SubmitOrder{
			Symbol: "ETHUSD",
			Side:   types.SideTypeSell,
			Type:   types.OrderTypeLimit,
		}

		rawOrder := &api.CreateOrderResponse{
			ID:        "order-uuid-456",
			ProductID: "ETH-USD",
			Side:      api.SideTypeSell,
		}

		store.add(submitOrder, rawOrder)

		activeOrder, ok := store.get("order-uuid-456")
		assert.True(t, ok)
		assert.NotNil(t, activeOrder)
		assert.Equal(t, "ETHUSD", activeOrder.submitOrder.Symbol)
		assert.Equal(t, "order-uuid-456", activeOrder.rawOrder.ID)
	})

	t.Run("non-existing order", func(t *testing.T) {
		activeOrder, ok := store.get("non-existent-uuid")
		assert.False(t, ok)
		assert.Nil(t, activeOrder)
	})
}

func TestActiveOrderStore_RemoveByUUID(t *testing.T) {
	store := newActiveOrderStore("test-key-4")

	submitOrder := types.SubmitOrder{
		Symbol: "BTCUSD",
		Side:   types.SideTypeBuy,
	}

	rawOrder := &api.CreateOrderResponse{
		ID:        "order-to-remove",
		ProductID: "BTC-USD",
		Status:    api.OrderStatusOpen,
	}

	store.add(submitOrder, rawOrder)
	assert.Equal(t, 1, len(store.orders))

	store.markCanceled("order-to-remove")
	// The order should still be in the store
	assert.Equal(t, 1, len(store.orders))

	// The order should be marked as canceled
	activeOrder, ok := store.get("order-to-remove")
	assert.True(t, ok)
	assert.NotNil(t, activeOrder)
	assert.Equal(t, api.OrderStatusCanceled, activeOrder.rawOrder.Status)
}

func TestActiveOrderStore_RemoveByUUID_NonExistent(t *testing.T) {
	store := newActiveOrderStore("test-key-5")

	// Marking a non-existent order as canceled should not cause issues
	store.markCanceled("non-existent-order")
	assert.Equal(t, 0, len(store.orders))
}

func TestActiveOrderStore_MultipleOrders(t *testing.T) {
	store := newActiveOrderStore("test-key-6")

	// Add multiple orders
	for i := 1; i <= 5; i++ {
		submitOrder := types.SubmitOrder{
			Symbol: "BTCUSD",
			Side:   types.SideTypeBuy,
		}

		rawOrder := &api.CreateOrderResponse{
			ID:        fixedpoint.NewFromInt(int64(i)).String(),
			ProductID: "BTC-USD",
			Status:    api.OrderStatusOpen,
		}

		store.add(submitOrder, rawOrder)
	}

	assert.Equal(t, 5, len(store.orders))

	// Mark one order as canceled
	store.markCanceled("2")
	// markCanceled updates status but doesn't remove orders from the store
	assert.Equal(t, 5, len(store.orders))

	// Verify the canceled order is marked as canceled
	canceledOrder, ok := store.get("2")
	assert.True(t, ok)
	assert.Equal(t, api.OrderStatusCanceled, canceledOrder.rawOrder.Status)

	// Verify other orders still exist and are still open
	for _, id := range []string{"1", "3", "4", "5"} {
		order, ok := store.get(id)
		assert.True(t, ok)
		assert.Equal(t, api.OrderStatusOpen, order.rawOrder.Status)
	}
}

func TestActiveOrderStore_ThreadSafety(t *testing.T) {
	store := newActiveOrderStore("test-key-7")
	var wg sync.WaitGroup

	// Number of concurrent operations
	numGoroutines := 100
	numOpsPerGoroutine := 100

	// Concurrent adds
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				orderID := fixedpoint.NewFromInt(int64(goroutineID*numOpsPerGoroutine + j)).String()
				submitOrder := types.SubmitOrder{
					Symbol: "BTCUSD",
					Side:   types.SideTypeBuy,
				}
				rawOrder := &api.CreateOrderResponse{
					ID:        orderID,
					ProductID: "BTC-USD",
				}
				store.add(submitOrder, rawOrder)
			}
		}(i)
	}
	wg.Wait()

	assert.Equal(t, numGoroutines*numOpsPerGoroutine, len(store.orders))

	// Concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				orderID := fixedpoint.NewFromInt(int64(goroutineID*numOpsPerGoroutine + j)).String()
				_, ok := store.get(orderID)
				assert.True(t, ok)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent removes (marking as canceled)
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				orderID := fixedpoint.NewFromInt(int64(goroutineID*numOpsPerGoroutine + j)).String()
				store.markCanceled(orderID)
			}
		}(i)
	}
	wg.Wait()

	// All orders should still be in the store, but marked as canceled
	assert.Equal(t, numGoroutines*numOpsPerGoroutine, len(store.orders))

	// Verify all orders are marked as canceled
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numOpsPerGoroutine; j++ {
			orderID := fixedpoint.NewFromInt(int64(i*numOpsPerGoroutine + j)).String()
			order, ok := store.get(orderID)
			assert.True(t, ok)
			assert.Equal(t, api.OrderStatusCanceled, order.rawOrder.Status)
		}
	}
}

func TestActiveOrderStore_OverwriteOrder(t *testing.T) {
	store := newActiveOrderStore("test-key-8")

	// Add an order
	submitOrder1 := types.SubmitOrder{
		Symbol: "BTCUSD",
		Side:   types.SideTypeBuy,
		Price:  fixedpoint.NewFromFloat(50000),
	}

	rawOrder1 := &api.CreateOrderResponse{
		ID:        "duplicate-uuid",
		ProductID: "BTC-USD",
		Price:     fixedpoint.NewFromFloat(50000),
	}

	store.add(submitOrder1, rawOrder1)

	// Add another order with the same UUID (should overwrite)
	submitOrder2 := types.SubmitOrder{
		Symbol: "ETHUSD",
		Side:   types.SideTypeSell,
		Price:  fixedpoint.NewFromFloat(3000),
	}

	rawOrder2 := &api.CreateOrderResponse{
		ID:        "duplicate-uuid",
		ProductID: "ETH-USD",
		Price:     fixedpoint.NewFromFloat(3000),
	}

	store.add(submitOrder2, rawOrder2)

	// Should still have only one order
	assert.Equal(t, 1, len(store.orders))

	// Should have the second order's data
	activeOrder, ok := store.get("duplicate-uuid")
	assert.True(t, ok)
	assert.Equal(t, "ETHUSD", activeOrder.submitOrder.Symbol)
	assert.Equal(t, fixedpoint.NewFromFloat(3000), activeOrder.rawOrder.Price)
}

func TestActiveOrder_Fields(t *testing.T) {
	submitOrder := types.SubmitOrder{
		Symbol:        "BTCUSD",
		Side:          types.SideTypeBuy,
		Type:          types.OrderTypeLimit,
		Quantity:      fixedpoint.NewFromFloat(0.5),
		Price:         fixedpoint.NewFromFloat(45000),
		Market:        types.Market{Symbol: "BTCUSD"},
		TimeInForce:   types.TimeInForceGTC,
		ClientOrderID: "client-order-123",
	}

	rawOrder := &api.CreateOrderResponse{
		ID:            "order-uuid-789",
		ProductID:     "BTC-USD",
		Side:          api.SideTypeBuy,
		Type:          api.OrderTypeLimit,
		Price:         fixedpoint.NewFromFloat(45000),
		Size:          fixedpoint.NewFromFloat(0.5),
		ClientOrderID: "client-order-123",
		Status:        api.OrderStatusOpen,
	}

	activeOrder := &ActiveOrder{
		submitOrder: submitOrder,
		rawOrder:    rawOrder,
	}

	assert.Equal(t, "BTCUSD", activeOrder.submitOrder.Symbol)
	assert.Equal(t, types.SideTypeBuy, activeOrder.submitOrder.Side)
	assert.Equal(t, types.OrderTypeLimit, activeOrder.submitOrder.Type)
	assert.Equal(t, fixedpoint.NewFromFloat(0.5), activeOrder.submitOrder.Quantity)
	assert.Equal(t, fixedpoint.NewFromFloat(45000), activeOrder.submitOrder.Price)
	assert.Equal(t, "client-order-123", activeOrder.submitOrder.ClientOrderID)

	assert.Equal(t, "order-uuid-789", activeOrder.rawOrder.ID)
	assert.Equal(t, "BTC-USD", activeOrder.rawOrder.ProductID)
	assert.Equal(t, api.SideTypeBuy, activeOrder.rawOrder.Side)
	assert.Equal(t, api.OrderTypeLimit, activeOrder.rawOrder.Type)
	assert.Equal(t, fixedpoint.NewFromFloat(45000), activeOrder.rawOrder.Price)
	assert.Equal(t, fixedpoint.NewFromFloat(0.5), activeOrder.rawOrder.Size)
	assert.Equal(t, "client-order-123", activeOrder.rawOrder.ClientOrderID)
	assert.Equal(t, api.OrderStatusOpen, activeOrder.rawOrder.Status)
}

func TestActiveOrderStore_Purge(t *testing.T) {
	store := newActiveOrderStore("test-key-purge")

	now := time.Now()

	// Add orders with different statuses
	orders := []struct {
		id         string
		status     api.OrderStatus
		lastUpdate time.Time
	}{
		{"canceled-order", api.OrderStatusCanceled, now},
		{"done-order", api.OrderStatusDone, now},
		{"rejected-order", api.OrderStatusRejected, now},
		{"open-order", api.OrderStatusOpen, now},
		{"expired-order", api.OrderStatusOpen, now.Add(-4 * time.Hour)}, // Older than 3 hours
		{"recent-order", api.OrderStatusOpen, now.Add(-2 * time.Hour)},  // Within 3 hours
	}

	for _, o := range orders {
		submitOrder := types.SubmitOrder{
			Symbol: "BTCUSD",
			Side:   types.SideTypeBuy,
		}
		rawOrder := &api.CreateOrderResponse{
			ID:     o.id,
			Status: o.status,
		}
		store.add(submitOrder, rawOrder)
		// Manually set lastUpdate time
		if activeOrder, ok := store.get(o.id); ok {
			activeOrder.lastUpdate = o.lastUpdate
		}
	}

	assert.Equal(t, 6, len(store.orders))

	// Run purge
	store.purge()

	// After purge, should have removed:
	// - canceled-order (status: canceled)
	// - done-order (status: done)
	// - rejected-order (status: rejected)
	// - expired-order (status: open but older than 3 hours)
	// Should keep:
	// - open-order (status: open, recent)
	// - recent-order (status: open, within 3 hours)
	assert.Equal(t, 2, len(store.orders))

	// Verify the remaining orders
	_, ok := store.get("open-order")
	assert.True(t, ok, "open-order should still exist")

	_, ok = store.get("recent-order")
	assert.True(t, ok, "recent-order should still exist")

	// Verify removed orders
	_, ok = store.get("canceled-order")
	assert.False(t, ok, "canceled-order should be removed")

	_, ok = store.get("done-order")
	assert.False(t, ok, "done-order should be removed")

	_, ok = store.get("rejected-order")
	assert.False(t, ok, "rejected-order should be removed")

	_, ok = store.get("expired-order")
	assert.False(t, ok, "expired-order should be removed")
}

func TestActiveOrderStore_Purge_EmptyStore(t *testing.T) {
	store := newActiveOrderStore("test-key-purge-empty")

	// Purge on empty store should not panic
	assert.Equal(t, 0, len(store.orders))
	store.purge()
	assert.Equal(t, 0, len(store.orders))
}

func TestActiveOrderStore_Purge_AllExpired(t *testing.T) {
	store := newActiveOrderStore("test-key-purge-all-expired")

	now := time.Now()
	oldTime := now.Add(-4 * time.Hour)

	// Add only expired orders
	for i := 1; i <= 3; i++ {
		submitOrder := types.SubmitOrder{
			Symbol: "BTCUSD",
			Side:   types.SideTypeBuy,
		}
		rawOrder := &api.CreateOrderResponse{
			ID:     fixedpoint.NewFromInt(int64(i)).String(),
			Status: api.OrderStatusOpen,
		}
		store.add(submitOrder, rawOrder)
		// Manually set lastUpdate time to old
		if activeOrder, ok := store.get(fixedpoint.NewFromInt(int64(i)).String()); ok {
			activeOrder.lastUpdate = oldTime
		}
	}

	assert.Equal(t, 3, len(store.orders))

	// Run purge
	store.purge()

	// All orders should be removed
	assert.Equal(t, 0, len(store.orders))
}

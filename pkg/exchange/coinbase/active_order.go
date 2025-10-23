package coinbase

import (
	"context"
	"sync"
	"time"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/types"
)

type ActiveOrder struct {
	submitOrder types.SubmitOrder
	rawOrder    *api.CreateOrderResponse

	lastUpdate time.Time
}

var actStoreRegistry sync.Map

type ActiveOrderStore struct {
	mu     sync.Mutex
	orders map[string]*ActiveOrder

	cleanWorkerOnce sync.Once
}

func newActiveOrderStore(key string) *ActiveOrderStore {
	if store, ok := actStoreRegistry.Load(key); ok {
		return store.(*ActiveOrderStore)
	}
	store := &ActiveOrderStore{
		orders: make(map[string]*ActiveOrder),
	}
	actStoreRegistry.Store(key, store)
	return store
}

// Start starts the active order store cleanup worker.
func (a *ActiveOrderStore) Start(ctx context.Context) {
	// Ensure the cleanup worker is started only once.
	a.cleanWorkerOnce.Do(func() {
		go a.cleanupWorker(ctx)
	})

}

func (a *ActiveOrderStore) cleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("active order store cleanup worker stopped")
			return
		case <-ticker.C:
			a.purge()
		}
	}
}

// purge removes orders that are completed or expired from the active order store.
// completed orders are those with status Canceled, Done, or Rejected.
// expired orders are those that have not been updated for more than 3 hours.
func (a *ActiveOrderStore) purge() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for orderUUID, activeOrder := range a.orders {
		switch activeOrder.rawOrder.Status {
		case api.OrderStatusCanceled, api.OrderStatusDone, api.OrderStatusRejected:
			logger.Infof(
				"removing %s order from active order store: %s",
				activeOrder.rawOrder.Status,
				orderUUID,
			)
			delete(a.orders, orderUUID)
		default:
			if time.Since(activeOrder.lastUpdate) > time.Hour*3 {
				logger.Infof("removing expired order from active order store: %s", orderUUID)
				delete(a.orders, orderUUID)
			}
		}
	}
}

func (a *ActiveOrderStore) get(orderUUID string) (*ActiveOrder, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	order, ok := a.orders[orderUUID]
	return order, ok
}

func (a *ActiveOrderStore) add(order types.SubmitOrder, rawOrder *api.CreateOrderResponse) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.orders[rawOrder.ID] = &ActiveOrder{
		submitOrder: order,
		rawOrder:    rawOrder,

		lastUpdate: time.Now(),
	}
}

func (a *ActiveOrderStore) markCanceled(orderUUID string) {
	a.update(
		orderUUID,
		&api.CreateOrderResponse{
			Status: api.OrderStatusCanceled,
		},
	)
}

func (a *ActiveOrderStore) update(orderUUID string, update *api.CreateOrderResponse) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	if activeOrder, ok := a.orders[orderUUID]; ok {
		activeOrder.rawOrder.Status = update.Status
		// filled size should be increasing
		if update.FilledSize.Compare(activeOrder.rawOrder.FilledSize) > 0 {
			activeOrder.rawOrder.FilledSize = update.FilledSize
		}
		activeOrder.lastUpdate = now
	}
}

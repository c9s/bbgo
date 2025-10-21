package coinbase

import (
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

type ActiveOrderStore struct {
	mu     sync.Mutex
	orders map[string]*ActiveOrder
}

func newActiveOrderStore() *ActiveOrderStore {
	store := &ActiveOrderStore{
		orders: make(map[string]*ActiveOrder),
	}
	go store.cleanupWorker()
	return store
}

func (a *ActiveOrderStore) cleanupWorker() {
	ticker := time.NewTicker(time.Hour * 2)
	for range ticker.C {
		a.mu.Lock()
		for orderUUID, activeOrder := range a.orders {
			if activeOrder.rawOrder.Status == api.OrderStatusCanceled ||
				activeOrder.rawOrder.Status == api.OrderStatusDone ||
				activeOrder.rawOrder.Status == api.OrderStatusRejected ||
				time.Since(activeOrder.lastUpdate) > time.Hour*3 {
				delete(a.orders, orderUUID)
			}
		}
		a.mu.Unlock()
	}
}

func (a *ActiveOrderStore) getByUUID(orderUUID string) (*ActiveOrder, bool) {
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

func (a *ActiveOrderStore) removeByUUID(orderUUID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.orders, orderUUID)
}

func (a *ActiveOrderStore) update(orderUUID string, update *api.CreateOrderResponse) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	if activeOrder, ok := a.orders[orderUUID]; ok {
		activeOrder.rawOrder.Status = update.Status
		activeOrder.lastUpdate = now
	}
}

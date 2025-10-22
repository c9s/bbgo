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

	startTime time.Time
}

func newActiveOrderStore() *ActiveOrderStore {
	store := &ActiveOrderStore{
		orders: make(map[string]*ActiveOrder),
	}
	return store
}

func (a *ActiveOrderStore) IsStarted() bool {
	return !a.startTime.IsZero()
}

func (a *ActiveOrderStore) Start() {
	if a.startTime.IsZero() {
		a.mu.Lock()
		a.startTime = time.Now()
		a.mu.Unlock()
		go a.cleanupWorker()
	}
}

func (a *ActiveOrderStore) cleanupWorker() {
	ticker := time.NewTicker(time.Minute * 5)
	for range ticker.C {
		a.mu.Lock()
		for orderUUID, activeOrder := range a.orders {
			switch {
			case activeOrder.rawOrder.Status == api.OrderStatusCanceled:
				coinbaseLogger.Infof("removing canceled order from active order store: %s", orderUUID)
				delete(a.orders, orderUUID)
			case activeOrder.rawOrder.Status == api.OrderStatusDone:
				coinbaseLogger.Infof("removing done order from active order store: %s", orderUUID)
				delete(a.orders, orderUUID)
			case activeOrder.rawOrder.Status == api.OrderStatusRejected:
				coinbaseLogger.Infof("removing rejected order from active order store: %s", orderUUID)
				delete(a.orders, orderUUID)
			case time.Since(activeOrder.lastUpdate) > time.Hour*3:
				coinbaseLogger.Infof("removing expired order from active order store: %s", orderUUID)
				delete(a.orders, orderUUID)
			default:
				continue
			}
		}
		a.mu.Unlock()
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

func (a *ActiveOrderStore) remove(orderUUID string) {
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
		activeOrder.lastUpdate = now
	}
}

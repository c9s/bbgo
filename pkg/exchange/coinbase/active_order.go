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

type registryKey struct {
	key, secret, passphrase string
}

var actStoreRegistry map[registryKey]*ActiveOrderStore = make(map[registryKey]*ActiveOrderStore)

type ActiveOrderStore struct {
	mu     sync.Mutex
	orders map[string]*ActiveOrder

	startTime time.Time
	ctx       context.Context
	cancel    context.CancelFunc
}

func newActiveOrderStore(key, secret, passphrase string) *ActiveOrderStore {
	rk := registryKey{key, secret, passphrase}
	if store, ok := actStoreRegistry[rk]; ok {
		return store
	}
	store := &ActiveOrderStore{
		orders: make(map[string]*ActiveOrder),
	}
	store.ctx, store.cancel = context.WithCancel(context.Background())
	actStoreRegistry[rk] = store
	return store
}

func (a *ActiveOrderStore) IsStarted() bool {
	return !a.startTime.IsZero()
}

// Start starts the active order store cleanup worker.
// **IMPORTANT**: Should be called only once per store instance.
func (a *ActiveOrderStore) Start() {
	if a.startTime.IsZero() {
		a.mu.Lock()
		a.startTime = time.Now()
		a.mu.Unlock()
		go a.cleanupWorker(a.ctx)
	}
}

// Stop stops the active order store cleanup worker.
// **IMPORTANT**: Should be called only once per store instance.
func (a *ActiveOrderStore) Stop() {
	if a.startTime.IsZero() {
		return
	}
	a.cancel()
}

func (a *ActiveOrderStore) cleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 5)
	for {
		select {
		case <-ctx.Done():
			coinbaseLogger.Info("active order store cleanup worker stopped")
			return
		case <-ticker.C:
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

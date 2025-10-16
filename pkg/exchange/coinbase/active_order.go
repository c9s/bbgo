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
}

type ActiveOrderStore struct {
	activeOrderMutex sync.Mutex
	activeOrders     map[string]*ActiveOrder
}

func newActiveOrderStore(e *Exchange) *ActiveOrderStore {
	store := &ActiveOrderStore{
		activeOrders: make(map[string]*ActiveOrder),
	}
	go store.trimActiveOrdersWorker(e)
	return store
}

func (a *ActiveOrderStore) getActiveOrderByUUID(orderUUID string) (*ActiveOrder, bool) {
	a.activeOrderMutex.Lock()
	defer a.activeOrderMutex.Unlock()

	order, ok := a.activeOrders[orderUUID]
	return order, ok
}

func (a *ActiveOrderStore) addActiveOrder(order types.SubmitOrder, rawOrder *api.CreateOrderResponse) {
	a.activeOrderMutex.Lock()
	defer a.activeOrderMutex.Unlock()

	a.activeOrders[rawOrder.ID] = &ActiveOrder{
		submitOrder: order,
		rawOrder:    rawOrder,
	}
}

func (a *ActiveOrderStore) removeActiveOrderByUUID(orderUUID string) {
	a.activeOrderMutex.Lock()
	defer a.activeOrderMutex.Unlock()

	delete(a.activeOrders, orderUUID)
}

func (a *ActiveOrderStore) trimActiveOrdersWorker(e *Exchange) {
	// trim the active orders every 30 minutes
	ticker := time.NewTicker(time.Minute * 30)
	defer ticker.Stop()

	for range ticker.C {
		func() {
			a.activeOrderMutex.Lock()
			defer a.activeOrderMutex.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			openOrders, err := e.QueryOpenOrders(ctx, "")
			if err != nil {
				return
			}
			openOrderIDs := make(map[string]struct{})
			for _, order := range openOrders {
				openOrderIDs[order.UUID] = struct{}{}
			}
			for orderID := range a.activeOrders {
				if _, ok := openOrderIDs[orderID]; !ok {
					delete(a.activeOrders, orderID)
				}
			}
		}()
	}
}

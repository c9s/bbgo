package coinbase

import (
	"sync"

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

func newActiveOrderStore() *ActiveOrderStore {
	return &ActiveOrderStore{
		activeOrders: make(map[string]*ActiveOrder),
	}
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

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
	mu     sync.Mutex
	orders map[string]*ActiveOrder
}

func newActiveOrderStore() *ActiveOrderStore {
	return &ActiveOrderStore{
		orders: make(map[string]*ActiveOrder),
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
	}
}

func (a *ActiveOrderStore) removeByUUID(orderUUID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.orders, orderUUID)
}

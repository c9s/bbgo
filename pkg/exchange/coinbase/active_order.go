package coinbase

import (
	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/types"
)

type ActiveOrder struct {
	createdOrder *types.Order
	rawOrder     *api.CreateOrderResponse
}

func (e *Exchange) addActiveOrder(order *types.Order, rawOrder *api.CreateOrderResponse) {
	e.activeOrders[rawOrder.ID] = &ActiveOrder{
		createdOrder: order,
		rawOrder:     rawOrder,
	}
}

func (e *Exchange) removeActiveOrderByUUID(orderUUID string) {
	delete(e.activeOrders, orderUUID)
}

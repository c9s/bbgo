package coinbase

import (
	"context"
	"time"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/types"
)

type ActiveOrder struct {
	createdOrder *types.Order
	rawOrder     *api.CreateOrderResponse
}

func (e *Exchange) addActiveOrder(order *types.Order, rawOrder *api.CreateOrderResponse) {
	e.activeOrderMutex.Lock()
	defer e.activeOrderMutex.Unlock()

	e.activeOrders[rawOrder.ID] = &ActiveOrder{
		createdOrder: order,
		rawOrder:     rawOrder,
	}
}

func (e *Exchange) removeActiveOrderByUUID(orderUUID string) {
	e.activeOrderMutex.Lock()
	defer e.activeOrderMutex.Unlock()

	delete(e.activeOrders, orderUUID)
}

func (e *Exchange) trimActiveOrdersWorker() {
	ticker := time.NewTicker(time.Minute * 30)
	defer ticker.Stop()

	for range ticker.C {
		func() {
			e.activeOrderMutex.Lock()
			defer e.activeOrderMutex.Unlock()
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
			for orderID := range e.activeOrders {
				if _, ok := openOrderIDs[orderID]; !ok {
					delete(e.activeOrders, orderID)
				}
			}
		}()
	}
}

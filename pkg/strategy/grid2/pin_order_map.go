package grid2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// PinOrderMap store the pin-order's relation, we will change key from string to fixedpoint.Value when FormatString fixed
type PinOrderMap map[string]types.Order

// AscendingOrders get the orders from pin order map and sort it in asc order
func (m PinOrderMap) AscendingOrders() []types.Order {
	var orders []types.Order
	for _, order := range m {
		// skip empty order
		if order.OrderID == 0 {
			continue
		}

		orders = append(orders, order)
	}

	types.SortOrdersUpdateTimeAscending(orders)

	return orders
}

func (m PinOrderMap) SyncOrderMap() *types.SyncOrderMap {
	orderMap := types.NewSyncOrderMap()
	for _, order := range m {
		orderMap.Add(order)
	}

	return orderMap
}

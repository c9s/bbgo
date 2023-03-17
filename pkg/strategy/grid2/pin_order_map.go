package grid2

import (
	"fmt"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// PinOrderMap store the pin-order's relation, we will change key from string to fixedpoint.Value when FormatString fixed
type PinOrderMap map[fixedpoint.Value]types.Order

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

func (m PinOrderMap) String() string {
	var sb strings.Builder

	sb.WriteString("================== PIN ORDER MAP ==================\n")
	for pin, order := range m {
		sb.WriteString(fmt.Sprintf("%+v -> %s\n", pin, order.String()))
	}
	sb.WriteString("================== END OF PIN ORDER MAP ==================\n")
	return sb.String()
}

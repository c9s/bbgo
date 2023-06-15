package grid2

import (
	"fmt"
	"sort"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// For grid trading, there are twin orders between a grid
// e.g. 100, 200, 300, 400, 500
//      BUY 100 and SELL 200 are a twin.
//      BUY 200 and SELL 300 are a twin.
// Because they can't be placed on orderbook at the same time

type TwinOrder struct {
	BuyOrder  types.Order
	SellOrder types.Order
}

func (t *TwinOrder) IsValid() bool {
	// XOR
	return (t.BuyOrder.OrderID == 0) != (t.SellOrder.OrderID == 0)
}

func (t *TwinOrder) Exist() bool {
	return t.BuyOrder.OrderID != 0 || t.SellOrder.OrderID != 0
}

func (t *TwinOrder) GetOrder() types.Order {
	if t.BuyOrder.OrderID != 0 {
		return t.BuyOrder
	}

	return t.SellOrder
}

func (t *TwinOrder) SetOrder(order types.Order) {
	if order.Side == types.SideTypeBuy {
		t.BuyOrder = order
		t.SellOrder = types.Order{}
	} else {
		t.SellOrder = order
		t.BuyOrder = types.Order{}
	}
}

type TwinOrderMap map[fixedpoint.Value]TwinOrder

func findTwinOrderMapKey(grid *Grid, order types.Order) (fixedpoint.Value, error) {
	if order.Side == types.SideTypeSell {
		return order.Price, nil
	}

	if order.Side == types.SideTypeBuy {
		pin, ok := grid.NextHigherPin(order.Price)
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("there is no next higher price for this order (%d, price: %s)", order.OrderID, order.Price)
		}

		return fixedpoint.Value(pin), nil
	}

	return fixedpoint.Zero, fmt.Errorf("unsupported side: %s of this order (%d)", order.Side, order.OrderID)
}

func (m TwinOrderMap) AscendingOrders() []types.Order {
	var orders []types.Order
	for _, twinOrder := range m {
		// skip empty order
		if !twinOrder.Exist() {
			continue
		}

		orders = append(orders, twinOrder.GetOrder())
	}

	types.SortOrdersUpdateTimeAscending(orders)

	return orders
}

func (m TwinOrderMap) SyncOrderMap() *types.SyncOrderMap {
	orderMap := types.NewSyncOrderMap()
	for _, twin := range m {
		orderMap.Add(twin.GetOrder())
	}

	return orderMap
}

func (m TwinOrderMap) String() string {
	var sb strings.Builder
	var pins []fixedpoint.Value
	for pin, _ := range m {
		pins = append(pins, pin)
	}

	sort.Slice(pins, func(i, j int) bool {
		return pins[j].Compare(pins[i]) < 0
	})

	sb.WriteString("================== TWIN ORDER MAP ==================\n")
	for _, pin := range pins {
		twin := m[pin]
		twinOrder := twin.GetOrder()
		sb.WriteString(fmt.Sprintf("-> %8s) %s\n", pin, twinOrder.String()))
	}
	sb.WriteString("================== END OF PIN ORDER MAP ==================\n")
	return sb.String()
}

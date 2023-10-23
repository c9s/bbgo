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

type TwinOrderBook struct {
	// sort in asc order
	pins []fixedpoint.Value

	// pin index, use to find the next or last pin in desc order
	pinIdx map[fixedpoint.Value]int

	// orderbook
	m map[fixedpoint.Value]*TwinOrder

	size int
}

func newTwinOrderBook(pins []Pin) *TwinOrderBook {
	var v []fixedpoint.Value
	for _, pin := range pins {
		v = append(v, fixedpoint.Value(pin))
	}

	// sort it in asc order
	sort.Slice(v, func(i, j int) bool {
		return v[j].Compare(v[i]) > 0
	})

	pinIdx := make(map[fixedpoint.Value]int)
	m := make(map[fixedpoint.Value]*TwinOrder)
	for i, pin := range v {
		m[pin] = &TwinOrder{}
		pinIdx[pin] = i
	}

	return &TwinOrderBook{
		pins:   v,
		pinIdx: pinIdx,
		m:      m,
		size:   0,
	}
}

func (b *TwinOrderBook) String() string {
	var sb strings.Builder

	sb.WriteString("================== TWIN ORDERBOOK ==================\n")
	for _, pin := range b.pins {
		twin := b.m[fixedpoint.Value(pin)]
		twinOrder := twin.GetOrder()
		sb.WriteString(fmt.Sprintf("-> %8s) %s\n", pin, twinOrder.String()))
	}
	sb.WriteString("================== END OF TWINORDERBOOK ==================\n")
	return sb.String()
}

func (b *TwinOrderBook) GetTwinOrderPin(order types.Order) (fixedpoint.Value, error) {
	idx, exist := b.pinIdx[order.Price]
	if !exist {
		return fixedpoint.Zero, fmt.Errorf("the order's (%d) price (%s) is not in pins", order.OrderID, order.Price)
	}

	if order.Side == types.SideTypeBuy {
		idx++
		if idx >= len(b.pins) {
			return fixedpoint.Zero, fmt.Errorf("this order's twin order price is not in pins, %+v", order)
		}
	} else if order.Side == types.SideTypeSell {
		if idx == 0 {
			return fixedpoint.Zero, fmt.Errorf("this order's twin order price is at zero index, %+v", order)
		}
		// do nothing
	} else {
		// should not happen
		return fixedpoint.Zero, fmt.Errorf("the order's (%d) side (%s) is not supported", order.OrderID, order.Side)
	}

	return b.pins[idx], nil
}

func (b *TwinOrderBook) AddOrder(order types.Order) error {
	pin, err := b.GetTwinOrderPin(order)
	if err != nil {
		return err
	}

	twinOrder, exist := b.m[pin]
	if !exist {
		// should not happen
		return fmt.Errorf("no any empty twin order at pins, should not happen, check it")
	}

	if !twinOrder.Exist() {
		b.size++
	}
	twinOrder.SetOrder(order)

	return nil
}

func (b *TwinOrderBook) GetTwinOrder(pin fixedpoint.Value) *TwinOrder {
	return b.m[pin]
}

func (b *TwinOrderBook) AddTwinOrder(pin fixedpoint.Value, order *TwinOrder) {
	b.m[pin] = order
}

func (b *TwinOrderBook) Size() int {
	return b.size
}

func (b *TwinOrderBook) EmptyTwinOrderSize() int {
	return len(b.pins) - 1 - b.size
}

func (b *TwinOrderBook) SyncOrderMap() *types.SyncOrderMap {
	orderMap := types.NewSyncOrderMap()
	for _, twin := range b.m {
		if twin.Exist() {
			orderMap.Add(twin.GetOrder())
		}
	}

	return orderMap
}

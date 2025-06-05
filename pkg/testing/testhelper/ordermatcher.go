package testhelper

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

type OrderMatcher struct {
	Order types.Order
}

func MatchOrder(o types.Order) *OrderMatcher {
	return &OrderMatcher{
		Order: o,
	}
}

func (m *OrderMatcher) Matches(x interface{}) bool {
	order, ok := x.(types.Order)
	if !ok {
		return false
	}

	return m.Order.OrderID == order.OrderID && m.Order.Price.Compare(m.Order.Price) == 0
}

func (m *OrderMatcher) String() string {
	return fmt.Sprintf("OrderMatcher expects %+v", m.Order)
}

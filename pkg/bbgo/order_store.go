package bbgo

import (
	"sync"

	"github.com/c9s/bbgo/pkg/types"
)

type OrderStore struct {
	// any created orders for tracking trades
	mu     sync.Mutex
	orders map[uint64]types.Order

	Symbol          string
	RemoveCancelled bool
	RemoveFilled    bool
	AddOrderUpdate  bool
}

func NewOrderStore(symbol string) *OrderStore {
	return &OrderStore{
		Symbol: symbol,
		orders: make(map[uint64]types.Order),
	}
}

func (s *OrderStore) AllFilled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If any order is new or partially filled, we return false
	for _, o := range s.orders {
		switch o.Status {

		case types.OrderStatusCanceled, types.OrderStatusRejected:
			continue

		case types.OrderStatusNew, types.OrderStatusPartiallyFilled:
			return false

		case types.OrderStatusFilled:
			// do nothing for the filled order

		}
	}

	// If we pass through the for loop, then all the orders filled
	return true
}

func (s *OrderStore) NumOfOrders() (num int) {
	s.mu.Lock()
	num = len(s.orders)
	s.mu.Unlock()
	return num
}

func (s *OrderStore) Orders() (orders []types.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, o := range s.orders {
		orders = append(orders, o)
	}

	return orders
}

func (s *OrderStore) Exists(oID uint64) (ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok = s.orders[oID]
	return ok
}

// Get a single order from the order store by order ID
// Should check ok to make sure the order is returned successfully
func (s *OrderStore) Get(oID uint64) (order types.Order, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok = s.orders[oID]
	return order, ok
}

func (s *OrderStore) Add(orders ...types.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, o := range orders {
		s.orders[o.OrderID] = o
	}
}

func (s *OrderStore) Remove(o types.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.orders, o.OrderID)
}

func (s *OrderStore) Update(o types.Order) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	old, ok := s.orders[o.OrderID]
	if ok {
		o.Tag = old.Tag
		s.orders[o.OrderID] = o
	}
	return ok
}

func (s *OrderStore) BindStream(stream types.Stream) {
	hasSymbol := s.Symbol != ""
	stream.OnOrderUpdate(func(order types.Order) {
		// if we have symbol defined, we should filter out the orders that we are not interested in
		if hasSymbol && order.Symbol != s.Symbol {
			return
		}

		s.handleOrderUpdate(order)
	})
}

func (s *OrderStore) handleOrderUpdate(order types.Order) {
	switch order.Status {

	case types.OrderStatusNew, types.OrderStatusPartiallyFilled, types.OrderStatusFilled:
		if s.AddOrderUpdate {
			s.Add(order)
		} else {
			s.Update(order)
		}

		if s.RemoveFilled && order.Status == types.OrderStatusFilled {
			s.Remove(order)
		}

	case types.OrderStatusCanceled:
		if s.RemoveCancelled {
			s.Remove(order)
		} else if order.ExecutedQuantity.IsZero() {
			s.Remove(order)
		}

	case types.OrderStatusRejected:
		s.Remove(order)
	}
}

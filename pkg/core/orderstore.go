package core

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

type OrderStore struct {
	// any created orders for tracking trades
	mu     sync.Mutex
	orders map[uint64]types.Order

	Symbol string

	// RemoveCancelled removes the canceled order when receiving a cancel order update event
	// It also removes the order even if it's partially filled
	// by default, only 0 filled canceled order will be removed.
	RemoveCancelled bool

	// RemoveFilled removes the fully filled order when receiving a filled order update event
	RemoveFilled bool

	// AddOrderUpdate adds the order into the store when receiving an order update when the order does not exist in the current store.
	AddOrderUpdate bool
	C              chan types.Order
}

func NewOrderStore(symbol string) *OrderStore {
	return &OrderStore{
		Symbol: symbol,
		orders: make(map[uint64]types.Order),
		C:      make(chan types.Order),
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
		old, ok := s.orders[o.OrderID]
		if ok && o.Tag == "" && old.Tag != "" {
			o.Tag = old.Tag
		}
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

	existing, ok := s.orders[o.OrderID]
	if ok {
		existing.Update(o)
		s.orders[o.OrderID] = existing
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

		s.HandleOrderUpdate(order)
	})
}

func (s *OrderStore) Size() (l int) {
	s.mu.Lock()
	l = len(s.orders)
	s.mu.Unlock()
	return l
}

func (s *OrderStore) Prune(expiryDuration time.Duration) {
	size := s.Size()

	// skip prune if the size is less than 100
	if size < 100 {
		return
	}

	cutOffTime := time.Now().Add(-expiryDuration)
	orders := make(map[uint64]types.Order, size)

	s.mu.Lock()
	defer s.mu.Unlock()

	logrus.Infof("orderStore: pruning orders before %s, %d orders", cutOffTime.String(), len(s.orders))

	for idx, o := range s.orders {
		// if the order is canceled or filled, we should remove the order if the update time is before the cut off time
		if o.Status == types.OrderStatusCanceled || o.Status == types.OrderStatusFilled {
			if o.UpdateTime.Time().Before(cutOffTime) {
				continue
			}
		}

		orders[idx] = o
	}

	s.orders = orders
}

func (s *OrderStore) HandleOrderUpdate(order types.Order) {

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

	select {
	case s.C <- order:
	default:
	}
}

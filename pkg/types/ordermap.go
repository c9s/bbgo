package types

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// OrderMap is used for storing orders by their order id
type OrderMap map[uint64]Order

func NewOrderMap(os ...Order) OrderMap {
	m := OrderMap{}
	if len(os) > 0 {
		m.Add(os...)
	}
	return m
}

func (m OrderMap) Backup() (orderForms []SubmitOrder) {
	for _, order := range m {
		orderForms = append(orderForms, order.Backup())
	}

	return orderForms
}

// Add the order the map
func (m OrderMap) Add(os ...Order) {
	for _, o := range os {
		m[o.OrderID] = o
	}
}

// Update only updates the order when the order ID exists in the map
func (m OrderMap) Update(o Order) {
	if _, ok := m[o.OrderID]; ok {
		m[o.OrderID] = o
	}
}

func (m OrderMap) Lookup(f func(o Order) bool) *Order {
	for _, order := range m {
		if f(order) {
			// copy and return
			o := order
			return &o
		}
	}
	return nil
}

func (m OrderMap) Remove(orderID uint64) {
	delete(m, orderID)
}

func (m OrderMap) IDs() (ids []uint64) {
	for id := range m {
		ids = append(ids, id)
	}

	return ids
}

func (m OrderMap) Exists(orderID uint64) bool {
	_, ok := m[orderID]
	return ok
}

func (m OrderMap) Get(orderID uint64) (Order, bool) {
	order, ok := m[orderID]
	return order, ok
}

func (m OrderMap) FindByStatus(status OrderStatus) (orders OrderSlice) {
	for _, o := range m {
		if o.Status == status {
			orders = append(orders, o)
		}
	}

	return orders
}

func (m OrderMap) Filled() OrderSlice {
	return m.FindByStatus(OrderStatusFilled)
}

func (m OrderMap) Canceled() OrderSlice {
	return m.FindByStatus(OrderStatusCanceled)
}

func (m OrderMap) Orders() (orders OrderSlice) {
	for _, o := range m {
		orders = append(orders, o)
	}
	return orders
}

type SyncOrderMap struct {
	orders OrderMap

	// pendingRemoval is for recording the order remove message for unknown orders.
	// the order removal message might arrive before the order update, so if we found there is a pending removal,
	// we should not keep the order in the order map
	pendingRemoval map[uint64]time.Time

	sync.RWMutex
}

func NewSyncOrderMap() *SyncOrderMap {
	return &SyncOrderMap{
		orders:         make(OrderMap),
		pendingRemoval: make(map[uint64]time.Time, 10),
	}
}

func (m *SyncOrderMap) Backup() (orders []SubmitOrder) {
	m.Lock()
	orders = m.orders.Backup()
	m.Unlock()
	return orders
}

func (m *SyncOrderMap) Remove(orderID uint64) (exists bool) {
	m.Lock()
	defer m.Unlock()

	exists = m.orders.Exists(orderID)
	if exists {
		m.orders.Remove(orderID)
	} else {
		m.pendingRemoval[orderID] = time.Now()
	}

	return exists
}

func (m *SyncOrderMap) processPendingRemoval() {
	m.Lock()
	defer m.Unlock()

	if len(m.pendingRemoval) == 0 {
		return
	}

	expireTime := time.Now().Add(-5 * time.Minute)
	removing := make(map[uint64]struct{})
	for orderID, creationTime := range m.pendingRemoval {
		if m.orders.Exists(orderID) || creationTime.Before(expireTime) {
			m.orders.Remove(orderID)
			removing[orderID] = struct{}{}
		}
	}

	for orderID := range removing {
		delete(m.pendingRemoval, orderID)
	}
}

func (m *SyncOrderMap) Add(o Order) {
	m.Lock()
	m.orders.Add(o)
	m.Unlock()

	m.processPendingRemoval()
}

func (m *SyncOrderMap) Update(o Order) {
	m.Lock()
	m.orders.Update(o)
	m.Unlock()
}

func (m *SyncOrderMap) Iterate(it func(id uint64, order Order) bool) {
	m.Lock()
	for id := range m.orders {
		if it(id, m.orders[id]) {
			break
		}
	}
	m.Unlock()
}

func (m *SyncOrderMap) Exists(orderID uint64) (exists bool) {
	m.Lock()
	exists = m.orders.Exists(orderID)
	m.Unlock()
	return exists
}

func (m *SyncOrderMap) Get(orderID uint64) (Order, bool) {
	m.Lock()
	order, ok := m.orders.Get(orderID)
	m.Unlock()
	return order, ok
}

func (m *SyncOrderMap) Lookup(f func(o Order) bool) *Order {
	m.Lock()
	defer m.Unlock()
	return m.orders.Lookup(f)
}

func (m *SyncOrderMap) Len() int {
	m.Lock()
	defer m.Unlock()
	return len(m.orders)
}

func (m *SyncOrderMap) IDs() (ids []uint64) {
	m.Lock()
	ids = m.orders.IDs()
	m.Unlock()
	return ids
}

func (m *SyncOrderMap) FindByStatus(status OrderStatus) OrderSlice {
	m.Lock()
	defer m.Unlock()

	return m.orders.FindByStatus(status)
}

func (m *SyncOrderMap) Filled() OrderSlice {
	return m.FindByStatus(OrderStatusFilled)
}

// AnyFilled find any order is filled and stop iterating the order map
func (m *SyncOrderMap) AnyFilled() (order Order, ok bool) {
	m.Lock()
	defer m.Unlock()

	for _, o := range m.orders {
		if o.Status == OrderStatusFilled {
			ok = true
			order = o
			return order, ok
		}
	}

	return
}

func (m *SyncOrderMap) Canceled() OrderSlice {
	return m.FindByStatus(OrderStatusCanceled)
}

func (m *SyncOrderMap) Orders() (slice OrderSlice) {
	m.RLock()
	slice = m.orders.Orders()
	m.RUnlock()
	return slice
}

type OrderSlice []Order

func (s *OrderSlice) Add(o Order) {
	*s = append(*s, o)
}

// Map builds up an OrderMap by the order id
func (s OrderSlice) Map() OrderMap {
	return NewOrderMap(s...)
}

func (s OrderSlice) SeparateBySide() (buyOrders, sellOrders []Order) {
	for _, o := range s {
		switch o.Side {
		case SideTypeBuy:
			buyOrders = append(buyOrders, o)
		case SideTypeSell:
			sellOrders = append(sellOrders, o)
		}
	}

	return buyOrders, sellOrders
}

func (s OrderSlice) Print() {
	for _, o := range s {
		logrus.Infof("%s", o)
	}
}

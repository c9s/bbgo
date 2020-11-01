package types

import log "github.com/sirupsen/logrus"

// LocalActiveOrderBook manages the local active order books.
type LocalActiveOrderBook struct {
	Bids *SyncOrderMap
	Asks *SyncOrderMap
}

func NewLocalActiveOrderBook() *LocalActiveOrderBook {
	return &LocalActiveOrderBook{
		Bids: NewSyncOrderMap(),
		Asks: NewSyncOrderMap(),
	}
}

func (b *LocalActiveOrderBook) Print() {
	for _, o := range b.Bids.Orders() {
		log.Infof("bid order: %d -> %s", o.OrderID, o.Status)
	}

	for _, o := range b.Asks.Orders() {
		log.Infof("ask order: %d -> %s", o.OrderID, o.Status)
	}
}

func (b *LocalActiveOrderBook) Add(orders ...Order) {
	for _, order := range orders {
		switch order.Side {
		case SideTypeBuy:
			b.Bids.Add(order)

		case SideTypeSell:
			b.Asks.Add(order)

		}
	}
}

func (b *LocalActiveOrderBook) NumOfBids() int {
	return b.Bids.Len()
}

func (b *LocalActiveOrderBook) NumOfAsks() int {
	return b.Asks.Len()
}

func (b *LocalActiveOrderBook) Delete(order Order) {
	switch order.Side {
	case SideTypeBuy:
		b.Bids.Delete(order.OrderID)

	case SideTypeSell:
		b.Asks.Delete(order.OrderID)

	}
}

// WriteOff writes off the filled order on the opposite side.
// This method does not write off order by order amount or order quantity.
func (b *LocalActiveOrderBook) WriteOff(order Order) bool {
	if order.Status != OrderStatusFilled {
		return false
	}

	switch order.Side {
	case SideTypeSell:
		// find the filled bid to remove
		if filledOrder, ok := b.Bids.AnyFilled(); ok {
			b.Bids.Delete(filledOrder.OrderID)
			b.Asks.Delete(order.OrderID)
			return true
		}

	case SideTypeBuy:
		// find the filled ask order to remove
		if filledOrder, ok := b.Asks.AnyFilled(); ok {
			b.Asks.Delete(filledOrder.OrderID)
			b.Bids.Delete(order.OrderID)
			return true
		}
	}

	return false
}

func (b *LocalActiveOrderBook) Orders() OrderSlice {
	return append(b.Asks.Orders(), b.Bids.Orders()...)
}

package bbgo

import (
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

// LocalActiveOrderBook manages the local active order books.
//go:generate callbackgen -type LocalActiveOrderBook
type LocalActiveOrderBook struct {
	Bids *types.SyncOrderMap
	Asks *types.SyncOrderMap

	filledCallbacks []func(o types.Order)
}

func NewLocalActiveOrderBook() *LocalActiveOrderBook {
	return &LocalActiveOrderBook{
		Bids: types.NewSyncOrderMap(),
		Asks: types.NewSyncOrderMap(),
	}
}

func (b *LocalActiveOrderBook) BindStream(stream types.Stream) {
	stream.OnOrderUpdate(b.orderUpdateHandler)
}

func (b *LocalActiveOrderBook) orderUpdateHandler(order types.Order) {
	log.Infof("[LocalActiveOrderBook] received order update: %+v", order)

	switch order.Status {
	case types.OrderStatusFilled:
		// make sure we have the order and we remove it
		if b.Remove(order) {
			b.EmitFilled(order)
		}

	case types.OrderStatusPartiallyFilled, types.OrderStatusNew:
		b.Update(order)

	case types.OrderStatusCanceled, types.OrderStatusRejected:
		log.Infof("[LocalActiveOrderBook] order status %s, removing %d...", order.Status, order.OrderID)
		b.Remove(order)
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

func (b *LocalActiveOrderBook) Update(orders ...types.Order) {
	for _, order := range orders {
		switch order.Side {
		case types.SideTypeBuy:
			b.Bids.Update(order)

		case types.SideTypeSell:
			b.Asks.Update(order)

		}
	}
}

func (b *LocalActiveOrderBook) Add(orders ...types.Order) {
	for _, order := range orders {
		switch order.Side {
		case types.SideTypeBuy:
			b.Bids.Add(order)

		case types.SideTypeSell:
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

func (b *LocalActiveOrderBook) Remove(order types.Order) bool {
	switch order.Side {
	case types.SideTypeBuy:
		return b.Bids.Remove(order.OrderID)

	case types.SideTypeSell:
		return b.Asks.Remove(order.OrderID)

	}

	return false
}

// WriteOff writes off the filled order on the opposite side.
// This method does not write off order by order amount or order quantity.
func (b *LocalActiveOrderBook) WriteOff(order types.Order) bool {
	if order.Status != types.OrderStatusFilled {
		return false
	}

	switch order.Side {
	case types.SideTypeSell:
		// find the filled bid to remove
		if filledOrder, ok := b.Bids.AnyFilled(); ok {
			b.Bids.Remove(filledOrder.OrderID)
			b.Asks.Remove(order.OrderID)
			return true
		}

	case types.SideTypeBuy:
		// find the filled ask order to remove
		if filledOrder, ok := b.Asks.AnyFilled(); ok {
			b.Asks.Remove(filledOrder.OrderID)
			b.Bids.Remove(order.OrderID)
			return true
		}
	}

	return false
}

func (b *LocalActiveOrderBook) Orders() types.OrderSlice {
	return append(b.Asks.Orders(), b.Bids.Orders()...)
}

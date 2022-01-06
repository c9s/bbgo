package bbgo

import (
	"context"
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

// LocalActiveOrderBook manages the local active order books.
//go:generate callbackgen -type LocalActiveOrderBook
type LocalActiveOrderBook struct {
	Symbol          string
	Asks, Bids      *types.SyncOrderMap
	filledCallbacks []func(o types.Order)
}

func NewLocalActiveOrderBook(symbol string) *LocalActiveOrderBook {
	return &LocalActiveOrderBook{
		Symbol: symbol,
		Bids:   types.NewSyncOrderMap(),
		Asks:   types.NewSyncOrderMap(),
	}
}

func (b *LocalActiveOrderBook) MarshalJSON() ([]byte, error) {
	orders := b.Backup()
	return json.Marshal(orders)
}

func (b *LocalActiveOrderBook) Backup() []types.SubmitOrder {
	return append(b.Bids.Backup(), b.Asks.Backup()...)
}

func (b *LocalActiveOrderBook) BindStream(stream types.Stream) {
	stream.OnOrderUpdate(b.orderUpdateHandler)
}

// GracefulCancel cancels the active orders gracefully
func (b *LocalActiveOrderBook) GracefulCancel(ctx context.Context, ex types.Exchange) error {
	if err := ex.CancelOrders(ctx, b.Orders()...); err != nil {
		log.WithError(err).Error("order cancel error")
	}

	// ensure every order is cancelled
	for b.NumOfOrders() > 0 {
		orders := b.Orders()
		log.Warnf("%d orders are not cancelled yet:", len(orders))
		b.Print()

		if err := ex.CancelOrders(ctx, b.Orders()...); err != nil {
			log.WithError(err).Errorf("can not cancel %s orders", b.Symbol)
			continue
		}

		log.Infof("waiting for orders to be cancelled...")

		// wait for 3 seconds to get the order updates
		select {
		case <-time.After(3 * time.Second):

		case <-ctx.Done():
			break

		}

		// verify the current open orders via the RESTful API
		if b.NumOfOrders() > 0 {
			log.Warnf("there are orders not cancelled, using REStful API to verify...")
			openOrders, err := ex.QueryOpenOrders(ctx, b.Symbol)
			if err != nil {
				log.WithError(err).Errorf("can not query %s open orders", b.Symbol)
				continue
			}

			openOrderStore := NewOrderStore(b.Symbol)
			openOrderStore.Add(openOrders...)

			for _, o := range b.Orders() {
				// if it does not exist, we should remove it
				if !openOrderStore.Exists(o.OrderID) {
					b.Remove(o)
				}
			}
		}
	}

	log.Info("all orders are cancelled successfully")
	return nil
}

func (b *LocalActiveOrderBook) orderUpdateHandler(order types.Order) {
	log.Debugf("[LocalActiveOrderBook] received order update: %+v", order)

	switch order.Status {
	case types.OrderStatusFilled:
		// make sure we have the order and we remove it
		if b.Remove(order) {
			b.EmitFilled(order)
		}

	case types.OrderStatusPartiallyFilled:
		b.Update(order)

	case types.OrderStatusNew:
		if order.Quantity == 0 {
			log.Debugf("[LocalActiveOrderBook] order status %s, removing order %s", order.Status, order)
			b.Remove(order)
		} else {
			b.Update(order)
		}

	case types.OrderStatusCanceled, types.OrderStatusRejected:
		log.Debugf("[LocalActiveOrderBook] order status %s, removing order %s", order.Status, order)
		b.Remove(order)

	default:
		log.Warnf("unhandled order status: %s", order.Status)
	}
}

func (b *LocalActiveOrderBook) Print() {
	for _, o := range b.Bids.Orders() {
		log.Infof("%s bid order: %d @ %f -> %s", o.Symbol, o.OrderID, o.Price, o.Status)
	}

	for _, o := range b.Asks.Orders() {
		log.Infof("%s ask order: %d @ %f -> %s", o.Symbol, o.OrderID, o.Price, o.Status)
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

func (b *LocalActiveOrderBook) Exists(order types.Order) bool {

	switch order.Side {

	case types.SideTypeBuy:
		return b.Bids.Exists(order.OrderID)

	case types.SideTypeSell:
		return b.Asks.Exists(order.OrderID)

	}

	return false
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

func (b *LocalActiveOrderBook) NumOfOrders() int {
	return b.Asks.Len() + b.Bids.Len()
}

func (b *LocalActiveOrderBook) Orders() types.OrderSlice {
	return append(b.Asks.Orders(), b.Bids.Orders()...)
}

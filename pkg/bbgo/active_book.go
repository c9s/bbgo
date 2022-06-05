package bbgo

import (
	"context"
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

const CancelOrderWaitTime = 20 * time.Millisecond

// LocalActiveOrderBook manages the local active order books.
//go:generate callbackgen -type LocalActiveOrderBook
type LocalActiveOrderBook struct {
	Symbol          string
	orders          *types.SyncOrderMap
	filledCallbacks []func(o types.Order)
}

func NewLocalActiveOrderBook(symbol string) *LocalActiveOrderBook {
	return &LocalActiveOrderBook{
		Symbol: symbol,
		orders: types.NewSyncOrderMap(),
	}
}

func (b *LocalActiveOrderBook) MarshalJSON() ([]byte, error) {
	orders := b.Backup()
	return json.Marshal(orders)
}

func (b *LocalActiveOrderBook) Backup() []types.SubmitOrder {
	return b.orders.Backup()
}

func (b *LocalActiveOrderBook) BindStream(stream types.Stream) {
	stream.OnOrderUpdate(b.orderUpdateHandler)
}

func (b *LocalActiveOrderBook) waitAllClear(ctx context.Context, waitTime, timeout time.Duration) (bool, error) {
	numOfOrders := b.NumOfOrders()
	clear := numOfOrders == 0
	if clear {
		return clear, nil
	}

	timeoutC := time.After(timeout)
	for {
		time.Sleep(waitTime)
		numOfOrders = b.NumOfOrders()
		clear = numOfOrders == 0
		select {
		case <-timeoutC:
			return clear, nil

		case <-ctx.Done():
			return clear, ctx.Err()

		default:
			if clear {
				return clear, nil
			}
		}
	}
}

// GracefulCancel cancels the active orders gracefully
func (b *LocalActiveOrderBook) GracefulCancel(ctx context.Context, ex types.Exchange) error {
	log.Debugf("[LocalActiveOrderBook] gracefully cancelling %s orders...", b.Symbol)

	startTime := time.Now()
	// ensure every order is cancelled
	for {
		orders := b.Orders()

		// Some orders in the variable are not created on the server side yet,
		// If we cancel these orders directly, we will get an unsent order error
		// We wait here for a while for server to create these orders.
		// time.Sleep(SentOrderWaitTime)

		// since ctx might be canceled, we should use background context here
		if err := ex.CancelOrders(context.Background(), orders...); err != nil {
			log.WithError(err).Errorf("[LocalActiveOrderBook] can not cancel %s orders", b.Symbol)
		}

		log.Debugf("[LocalActiveOrderBook] waiting %s for %s orders to be cancelled...", CancelOrderWaitTime, b.Symbol)

		clear, err := b.waitAllClear(ctx, CancelOrderWaitTime, 5*time.Second)
		if clear || err != nil {
			break
		}

		log.Warnf("[LocalActiveOrderBook] %d %s orders are not cancelled yet:", b.NumOfOrders(), b.Symbol)
		b.Print()

		// verify the current open orders via the RESTful API
		log.Warnf("[LocalActiveOrderBook] using REStful API to verify active orders...")
		openOrders, err := ex.QueryOpenOrders(ctx, b.Symbol)
		if err != nil {
			log.WithError(err).Errorf("can not query %s open orders", b.Symbol)
			continue
		}

		openOrderStore := NewOrderStore(b.Symbol)
		openOrderStore.Add(openOrders...)
		for _, o := range orders {
			// if it's not on the order book (open orders), we should remove it from our local side
			if !openOrderStore.Exists(o.OrderID) {
				b.Remove(o)
			}
		}
	}

	log.Debugf("[LocalActiveOrderBook] all %s orders are cancelled successfully in %s", b.Symbol, time.Since(startTime))
	return nil
}

func (b *LocalActiveOrderBook) orderUpdateHandler(order types.Order) {
	hasSymbol := len(b.Symbol) > 0
	if hasSymbol && order.Symbol != b.Symbol {
		return
	}

	log.Debugf("[LocalActiveOrderBook] received order update: %+v", order)

	switch order.Status {
	case types.OrderStatusFilled:
		// make sure we have the order and we remove it
		if b.Remove(order) {
			b.EmitFilled(order)
		}

	case types.OrderStatusPartiallyFilled, types.OrderStatusNew:
		b.Update(order)

	case types.OrderStatusCanceled, types.OrderStatusRejected:
		log.Debugf("[LocalActiveOrderBook] order status %s, removing order %s", order.Status, order)
		b.Remove(order)

	default:
		log.Warnf("unhandled order status: %s", order.Status)
	}
}

func (b *LocalActiveOrderBook) Print() {
	for _, o := range b.orders.Orders() {
		log.Infof("%s", o)
	}
}

func (b *LocalActiveOrderBook) Update(orders ...types.Order) {
	hasSymbol := len(b.Symbol) > 0
	for _, order := range orders {
		if hasSymbol && b.Symbol == order.Symbol {
			b.orders.Update(order)
		}
	}
}

func (b *LocalActiveOrderBook) Add(orders ...types.Order) {
	hasSymbol := len(b.Symbol) > 0
	for _, order := range orders {
		if hasSymbol && b.Symbol == order.Symbol {
			b.orders.Add(order)
		}
	}
}

func (b *LocalActiveOrderBook) Exists(order types.Order) bool {
	return b.orders.Exists(order.OrderID)
}

func (b *LocalActiveOrderBook) Remove(order types.Order) bool {
	return b.orders.Remove(order.OrderID)
}

func (b *LocalActiveOrderBook) NumOfOrders() int {
	return b.orders.Len()
}

func (b *LocalActiveOrderBook) Orders() types.OrderSlice {
	return b.orders.Orders()
}

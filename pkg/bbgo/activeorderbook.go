package bbgo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

const CancelOrderWaitTime = 20 * time.Millisecond

// ActiveOrderBook manages the local active order books.
//go:generate callbackgen -type ActiveOrderBook
type ActiveOrderBook struct {
	Symbol          string
	orders          *types.SyncOrderMap
	filledCallbacks []func(o types.Order)
}

func NewActiveOrderBook(symbol string) *ActiveOrderBook {
	return &ActiveOrderBook{
		Symbol: symbol,
		orders: types.NewSyncOrderMap(),
	}
}

func (b *ActiveOrderBook) MarshalJSON() ([]byte, error) {
	orders := b.Backup()
	return json.Marshal(orders)
}

func (b *ActiveOrderBook) Backup() []types.SubmitOrder {
	return b.orders.Backup()
}

func (b *ActiveOrderBook) BindStream(stream types.Stream) {
	stream.OnOrderUpdate(b.orderUpdateHandler)
}

func (b *ActiveOrderBook) waitClear(ctx context.Context, order types.Order, waitTime, timeout time.Duration) (bool, error) {
	if !b.Exists(order) {
		return true, nil
	}

	timeoutC := time.After(timeout)
	for {
		time.Sleep(waitTime)
		clear := !b.Exists(order)
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

func (b *ActiveOrderBook) waitAllClear(ctx context.Context, waitTime, timeout time.Duration) (bool, error) {
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

// Cancel orders without confirmation
func (b *ActiveOrderBook) CancelNoWait(ctx context.Context, ex types.Exchange, orders ...types.Order) error {
	// if no orders are given, set to cancelAll
	if len(orders) == 0 {
		orders = b.Orders()
	} else {
		// simple check on given input
		for _, o := range orders {
			if o.Symbol != b.Symbol {
				return errors.New("[ActiveOrderBook] cancel " + b.Symbol + " orderbook with different symbol: " + o.Symbol)
			}
		}
	}
	// optimize order cancel for back-testing
	if IsBackTesting {
		return ex.CancelOrders(context.Background(), orders...)
	}
	log.Debugf("[ActiveOrderBook] no wait cancelling %s orders...", b.Symbol)
	// since ctx might be canceled, we should use background context here
	if err := ex.CancelOrders(context.Background(), orders...); err != nil {
		log.WithError(err).Errorf("[ActiveOrderBook] no wait can not cancel %s orders", b.Symbol)
	}
	for _, o := range orders {
		b.Remove(o)
	}
	return nil
}

// GracefulCancel cancels the active orders gracefully
func (b *ActiveOrderBook) GracefulCancel(ctx context.Context, ex types.Exchange, orders ...types.Order) error {
	// if no orders are given, set to cancelAll
	if len(orders) == 0 {
		orders = b.Orders()
	} else {
		// simple check on given input
		for _, o := range orders {
			if b.Symbol != "" && o.Symbol != b.Symbol {
				return errors.New("[ActiveOrderBook] cancel " + b.Symbol + " orderbook with different symbol: " + o.Symbol)
			}
		}
	}
	// optimize order cancel for back-testing
	if IsBackTesting {
		return ex.CancelOrders(context.Background(), orders...)
	}

	log.Debugf("[ActiveOrderBook] gracefully cancelling %s orders...", b.Symbol)
	waitTime := CancelOrderWaitTime

	startTime := time.Now()
	// ensure every order is cancelled
	for {
		// Some orders in the variable are not created on the server side yet,
		// If we cancel these orders directly, we will get an unsent order error
		// We wait here for a while for server to create these orders.
		// time.Sleep(SentOrderWaitTime)

		// since ctx might be canceled, we should use background context here
		if err := ex.CancelOrders(context.Background(), orders...); err != nil {
			log.WithError(err).Errorf("[ActiveOrderBook] can not cancel %s orders", b.Symbol)
		}

		log.Debugf("[ActiveOrderBook] waiting %s for %s orders to be cancelled...", waitTime, b.Symbol)

		clear, err := b.waitAllClear(ctx, waitTime, 5*time.Second)
		if clear || err != nil {
			break
		}

		log.Warnf("[ActiveOrderBook] %d %s orders are not cancelled yet:", b.NumOfOrders(), b.Symbol)
		b.Print()

		// verify the current open orders via the RESTful API
		log.Warnf("[ActiveOrderBook] using REStful API to verify active orders...")

		var symbols = map[string]struct{}{}
		for _, order := range orders {
			symbols[order.Symbol] = struct{}{}

		}
		var leftOrders []types.Order

		for symbol := range symbols {
			openOrders, err := ex.QueryOpenOrders(ctx, symbol)
			if err != nil {
				log.WithError(err).Errorf("can not query %s open orders", symbol)
				continue
			}

			openOrderStore := NewOrderStore(symbol)
			openOrderStore.Add(openOrders...)
			for _, o := range orders {
				// if it's not on the order book (open orders), we should remove it from our local side
				if !openOrderStore.Exists(o.OrderID) {
					b.Remove(o)
				} else {
					leftOrders = append(leftOrders, o)
				}
			}
		}
		orders = leftOrders
	}

	log.Debugf("[ActiveOrderBook] all %s orders are cancelled successfully in %s", b.Symbol, time.Since(startTime))
	return nil
}

func (b *ActiveOrderBook) orderUpdateHandler(order types.Order) {
	hasSymbol := len(b.Symbol) > 0
	if hasSymbol && order.Symbol != b.Symbol {
		return
	}

	switch order.Status {
	case types.OrderStatusFilled:
		// make sure we have the order and we remove it
		if b.Remove(order) {
			b.EmitFilled(order)
		}

	case types.OrderStatusPartiallyFilled, types.OrderStatusNew:
		b.Update(order)

	case types.OrderStatusCanceled, types.OrderStatusRejected:
		log.Debugf("[ActiveOrderBook] order status %s, removing order %s", order.Status, order)
		b.Remove(order)

	default:
		log.Warnf("unhandled order status: %s", order.Status)
	}
}

func (b *ActiveOrderBook) Print() {
	for _, o := range b.orders.Orders() {
		log.Infof("%s", o)
	}
}

func (b *ActiveOrderBook) Update(orders ...types.Order) {
	hasSymbol := len(b.Symbol) > 0
	for _, order := range orders {
		if hasSymbol && b.Symbol == order.Symbol {
			b.orders.Update(order)
		}
	}
}

func (b *ActiveOrderBook) Add(orders ...types.Order) {
	hasSymbol := len(b.Symbol) > 0
	for _, order := range orders {
		if hasSymbol && b.Symbol == order.Symbol {
			b.orders.Add(order)
		}
	}
}

func (b *ActiveOrderBook) Exists(order types.Order) bool {
	return b.orders.Exists(order.OrderID)
}

func (b *ActiveOrderBook) Remove(order types.Order) bool {
	return b.orders.Remove(order.OrderID)
}

func (b *ActiveOrderBook) NumOfOrders() int {
	return b.orders.Len()
}

func (b *ActiveOrderBook) Orders() types.OrderSlice {
	return b.orders.Orders()
}

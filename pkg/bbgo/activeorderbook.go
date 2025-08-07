package bbgo

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/exchange"
	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/sigchan"
	"github.com/c9s/bbgo/pkg/types"
)

const DefaultCancelOrderWaitTime = 50 * time.Millisecond
const DefaultOrderCancelTimeout = 15 * time.Second

// ActiveOrderBook manages the local active order books.
//
//go:generate callbackgen -type ActiveOrderBook
type ActiveOrderBook struct {
	Symbol string

	orders              *types.SyncOrderMap
	pendingOrderUpdates *types.SyncOrderMap

	newCallbacks      []func(o types.Order)
	filledCallbacks   []func(o types.Order)
	canceledCallbacks []func(o types.Order)

	// sig is the order update signal
	// this signal will be emitted when a new order is added or removed.
	C sigchan.Chan

	mu sync.Mutex

	cancelOrderWaitTime time.Duration
	cancelOrderTimeout  time.Duration

	logger log.FieldLogger
}

func NewActiveOrderBook(symbol string) *ActiveOrderBook {
	logFields := log.Fields{}

	if symbol != "" {
		logFields["symbol"] = symbol
	}

	logger := log.WithFields(logFields)
	return &ActiveOrderBook{
		Symbol:              symbol,
		orders:              types.NewSyncOrderMap(),
		pendingOrderUpdates: types.NewSyncOrderMap(),
		C:                   sigchan.New(1),
		cancelOrderWaitTime: DefaultCancelOrderWaitTime,
		cancelOrderTimeout:  DefaultOrderCancelTimeout,
		logger:              logger,
	}
}

func (b *ActiveOrderBook) SetCancelOrderWaitTime(duration time.Duration) {
	b.cancelOrderWaitTime = duration
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

func (b *ActiveOrderBook) waitOrderClear(
	ctx context.Context, order types.Order, waitTime, timeout time.Duration,
) (bool, error) {
	if !b.orders.Exists(order.OrderID) {
		return true, nil
	}

	timeoutC := time.After(timeout)
	for {
		select {
		case <-time.After(waitTime):
		case <-b.C:
		}

		clear := !b.orders.Exists(order.OrderID)

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

// waitAllClear waits for the order book be clear (meaning every order is removed)
// if err != nil, it's the context error.
func (b *ActiveOrderBook) waitAllClear(ctx context.Context, waitTime, timeout time.Duration) (bool, error) {
	clear := b.NumOfOrders() == 0
	if clear {
		return clear, nil
	}

	timeoutC := time.After(timeout)
	for {
		select {
		case <-time.After(waitTime):
		case <-b.C:
		}

		// update clear flag
		clear = b.NumOfOrders() == 0

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

// FastCancel cancels the orders without verification
// It calls the exchange cancel order api and then remove the orders from the active orderbook directly.
func (b *ActiveOrderBook) FastCancel(ctx context.Context, ex types.Exchange, orders ...types.Order) error {
	// if no orders are given, set to cancelAll
	hasSymbol := b.Symbol != ""
	if len(orders) == 0 {
		orders = b.Orders()
	} else {
		// simple check on given input
		for _, o := range orders {
			if hasSymbol && o.Symbol != b.Symbol {
				return errors.New("[ActiveOrderBook] cancel " + b.Symbol + " orderbook with different order symbol: " + o.Symbol)
			}
		}
	}

	// optimize order cancel for back-testing
	if IsBackTesting {
		return ex.CancelOrders(ctx, orders...)
	}

	b.logger.Debugf("[ActiveOrderBook] no wait cancelling %s orders...", b.Symbol)
	if err := ex.CancelOrders(ctx, orders...); err != nil {
		b.logger.WithError(err).Errorf("[ActiveOrderBook] no wait can not cancel %s orders", b.Symbol)
	}

	for _, o := range orders {
		b.orders.Remove(o.OrderID)
	}
	return nil
}

// GracefulCancel cancels the active orders gracefully
func (b *ActiveOrderBook) GracefulCancel(ctx context.Context, ex types.Exchange, specifiedOrders ...types.Order) error {
	cancelAll := false
	orders := specifiedOrders

	// if no orders are given, set to cancelAll
	if len(specifiedOrders) == 0 {
		orders = b.Orders()
		cancelAll = true
	} else {
		// simple check on given input
		hasSymbol := b.Symbol != ""
		for _, o := range orders {
			if hasSymbol && o.Symbol != b.Symbol {
				return fmt.Errorf("[ActiveOrderBook] canceling %s orderbook with different symbol: %s", b.Symbol, o.Symbol)
			}
		}
	}

	// optimize order cancel for back-testing
	if IsBackTesting {
		return ex.CancelOrders(context.Background(), orders...)
	}

	b.logger.Debugf("[ActiveOrderBook] gracefully cancelling %s orders...", b.Symbol)
	waitTime := b.cancelOrderWaitTime

	startTime := time.Now()
	// ensure every order is canceled
	for {
		// Some orders in the variable are not created on the server side yet,
		// If we cancel these orders directly, we will get an unsent order error
		// We wait here for a while for server to create these orders.
		// time.Sleep(SentOrderWaitTime)

		// since ctx might be canceled, we should use background context here
		if err := ex.CancelOrders(context.Background(), orders...); err != nil {
			b.logger.WithError(err).Warnf("[ActiveOrderBook] can not cancel %d %s orders", len(orders), b.Symbol)
		}

		b.logger.Debugf("[ActiveOrderBook] waiting %s for %d %s orders to be cancelled...", waitTime, len(orders), b.Symbol)

		if cancelAll {
			clear, err := b.waitAllClear(ctx, waitTime, b.cancelOrderTimeout)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					b.logger.WithError(err).Errorf("order cancel error")
				}

				break
			}

			if clear {
				b.logger.Debugf("[ActiveOrderBook] %d %s orders are canceled", len(orders), b.Symbol)
				break
			}

			b.logger.Warnf("[ActiveOrderBook] %d/%d %s orders are not cancelled yet", b.NumOfOrders(), len(orders), b.Symbol)
			b.Print()

		} else {
			existingOrders := b.filterExistingOrders(orders)
			if len(existingOrders) == 0 {
				b.logger.Debugf("[ActiveOrderBook] orders are canceled")
				break
			}
		}

		// verify the current open orders via the RESTful API
		if orderQueryService, ok := ex.(types.ExchangeOrderQueryService); ok {
			for idx, o := range orders {
				retOrder, err := retry.QueryOrderUntilSuccessful(ctx, orderQueryService, o.AsQuery())

				if err != nil {
					b.logger.WithError(err).Errorf("unable to update order #%d", o.OrderID)
					continue
				} else if retOrder != nil {
					b.Update(*retOrder)

					orders[idx] = *retOrder
				}
			}

			if cancelAll {
				orders = b.Orders()
			} else {
				// for partial cancel
				orders = filterCanceledOrders(orders)
			}
		} else {
			b.logger.Warnf("[ActiveOrderBook] using open orders API to verify the active orders...")

			var symbolOrdersMap = categorizeOrderBySymbol(orders)
			var errOccurred bool
			var leftOrders types.OrderSlice
			for symbol, symbolOrders := range symbolOrdersMap {
				openOrders, err := ex.QueryOpenOrders(ctx, symbol)
				if err != nil {
					errOccurred = true
					b.logger.WithError(err).Errorf("can not query %s open orders", symbol)
					break
				}

				openOrderMap := types.NewOrderMap(openOrders...)
				for _, o := range symbolOrders {
					// if it's not on the order book (open orders),
					// we should remove it from our local side
					if !openOrderMap.Exists(o.OrderID) {
						b.Remove(o)
					} else {
						leftOrders.Add(o)
					}
				}
			}

			// if an error occurs, we cannot update the orders because it will result in an empty order slice.
			if !errOccurred {
				// update order slice for the next try
				orders = leftOrders
			}

		}
	}

	if cancelAll {
		b.clear()
	}

	b.logger.Debugf("[ActiveOrderBook] all %s orders are cancelled successfully in %s", b.Symbol, time.Since(startTime))
	return nil
}

func (b *ActiveOrderBook) clear() {
	b.orders.Clear()
	b.pendingOrderUpdates.Clear()
}

func (b *ActiveOrderBook) orderUpdateHandler(order types.Order) {
	if oldOrder, ok := b.Get(order.OrderID); ok {
		order.Tag = oldOrder.Tag
		order.GroupID = oldOrder.GroupID
	}
	b.Update(order)
}

func (b *ActiveOrderBook) Print() {
	orders := b.orders.Orders()
	orders = types.SortOrdersByPrice(orders, true)
	orders.Print()
}

// Update updates the order by the order status and emit the related events.
// When order is filled, the order will be removed from the internal order storage.
// When order is New or PartiallyFilled, the internal order will be updated according to the latest order update.
// When the order is cancelled, it will be removed from the internal order storage.
func (b *ActiveOrderBook) Update(order types.Order) {
	hasSymbol := len(b.Symbol) > 0
	if hasSymbol && order.Symbol != b.Symbol {
		return
	}

	b.mu.Lock()
	if !b.orders.Exists(order.OrderID) {
		b.logger.Debugf("[ActiveOrderBook] order #%d %s does not exist, adding it to pending order update", order.OrderID, order.Status)
		b.pendingOrderUpdates.Add(order)
		b.mu.Unlock()
		return
	}

	// if order update time is too old, skip it
	if previousOrder, ok := b.orders.Get(order.OrderID); ok {
		// the arguments ordering is important here
		// if we can't detect which is newer, isNewerOrderUpdate returns false
		// if you pass two same objects to isNewerOrderUpdate, it returns false
		if !isNewerOrderUpdate(order, previousOrder) {
			b.logger.Infof("[ActiveOrderBook] order #%d (update time %s) is out of date, skip it", order.OrderID, order.UpdateTime)
			b.mu.Unlock()
			return
		}
	}

	switch order.Status {
	case types.OrderStatusFilled:
		// make sure we have the order and we remove it
		removed := b.orders.Remove(order.OrderID)
		b.mu.Unlock()

		if removed {
			b.logger.Infof("[ActiveOrderBook] order #%d is filled: %s", order.OrderID, order.String())
			b.EmitFilled(order)
		}
		b.C.Emit()

	case types.OrderStatusPartiallyFilled:
		b.orders.Update(order)
		b.mu.Unlock()

	case types.OrderStatusNew:
		b.orders.Update(order)
		b.mu.Unlock()

		b.C.Emit()

	case types.OrderStatusCanceled, types.OrderStatusRejected, types.OrderStatusExpired:
		// TODO: note that orders transit to "canceled" may have partially filled
		b.logger.Debugf("[ActiveOrderBook] order is %s, removing order %s", order.Status, order)
		b.orders.Remove(order.OrderID)
		b.mu.Unlock()

		if order.Status == types.OrderStatusCanceled {
			b.EmitCanceled(order)
		}
		b.C.Emit()

	default:
		b.mu.Unlock()
		b.logger.Warnf("[ActiveOrderBook] unhandled order status: %s", order.Status)
	}
}

func (b *ActiveOrderBook) Add(orders ...types.Order) {
	hasSymbol := len(b.Symbol) > 0

	for _, order := range orders {
		if hasSymbol && b.Symbol != order.Symbol {
			continue
		}

		b.add(order)
	}
}

// isNewerOrderUpdate checks if the order a is newer than the order b.
func isNewerOrderUpdate(a, b types.Order) bool {
	// compare state first
	switch a.Status {

	case types.OrderStatusCanceled, types.OrderStatusRejected: // canceled is a final state
		switch b.Status {

		case types.OrderStatusCanceled, types.OrderStatusRejected, types.OrderStatusNew, types.OrderStatusPartiallyFilled:
			return true
		}

	case types.OrderStatusPartiallyFilled:
		switch b.Status {

		case types.OrderStatusNew:
			return true

		case types.OrderStatusPartiallyFilled:
			// unknown for equal
			if a.ExecutedQuantity.Compare(b.ExecutedQuantity) > 0 {
				return true
			}

			if a.UpdateTime.After(b.UpdateTime.Time()) {
				return true
			}
		}

	case types.OrderStatusFilled:
		switch b.Status {
		case types.OrderStatusFilled, types.OrderStatusPartiallyFilled, types.OrderStatusNew:
			return true
		}
	}

	return isNewerOrderUpdateTime(a, b)
}

func isNewerOrderUpdateTime(a, b types.Order) bool {
	au := time.Time(a.UpdateTime)
	bu := time.Time(b.UpdateTime)

	if !au.IsZero() && !bu.IsZero() && au.After(bu) {
		return true
	}

	if !au.IsZero() && bu.IsZero() {
		return true
	}

	return false
}

// add the order to the active order book and check the pending order
func (b *ActiveOrderBook) add(order types.Order) {
	if pendingOrder, ok := b.pendingOrderUpdates.Get(order.OrderID); ok {
		// if the pending order update time is newer than the adding order
		// we should use the pending order rather than the adding order.
		// if the pending order is older, then we should add the new one, and drop the pending order
		b.logger.Debugf("found pending order update: %+v", pendingOrder)
		if isNewerOrderUpdate(pendingOrder, order) {
			b.logger.Debugf("pending order update is newer: %+v", pendingOrder)
			if pendingOrder.Tag == "" {
				pendingOrder.Tag = order.Tag
				pendingOrder.GroupID = order.GroupID
			}
			order = pendingOrder
		}

		b.orders.Add(order)
		b.pendingOrderUpdates.Remove(pendingOrder.OrderID)
		b.EmitNew(order)

		// when using add(order), it's usually a new maker order on the order book.
		// so, when it's not status=new, we should trigger order update handler
		if order.Status != types.OrderStatusNew {
			// emit the order update handle function to trigger callback
			b.Update(order)
		}

	} else {
		b.orders.Add(order)
		b.EmitNew(order)
	}
}

func (b *ActiveOrderBook) Exists(orderID uint64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.orders.Exists(orderID)
}

func (b *ActiveOrderBook) Get(orderID uint64) (types.Order, bool) {
	return b.orders.Get(orderID)
}

func (b *ActiveOrderBook) Remove(order types.Order) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.orders.Remove(order.OrderID)
}

func (b *ActiveOrderBook) NumOfOrders() int {
	return b.orders.Len()
}

func (b *ActiveOrderBook) Orders() types.OrderSlice {
	return b.orders.Orders()
}

func (b *ActiveOrderBook) Lookup(f func(o types.Order) bool) *types.Order {
	return b.orders.Lookup(f)
}

func (b *ActiveOrderBook) filterExistingOrders(orders []types.Order) (existingOrders types.OrderSlice) {
	for _, o := range orders {
		// skip market order
		// this prevents if someone added a market order to the active order book
		if o.Type == types.OrderTypeMarket {
			continue
		}

		if b.Exists(o.OrderID) {
			existingOrders.Add(o)
		}
	}

	return existingOrders
}

func (b *ActiveOrderBook) SyncOrders(
	ctx context.Context, ex types.Exchange, bufferDuration time.Duration,
) (types.OrderSlice, error) {
	openOrders, err := retry.QueryOpenOrdersUntilSuccessfulLite(ctx, ex, b.Symbol)
	if err != nil {
		return nil, err
	}

	openOrdersMap := types.OrderSlice(openOrders).Map()

	// only sync orders which is updated over 3 min, because we may receive from websocket and handle it twice
	syncBefore := time.Now().Add(-bufferDuration)

	activeOrders := b.Orders()
	var errs error
	var updatedOrders types.OrderSlice

	// update active orders not in open orders
	for _, activeOrder := range activeOrders {
		if _, exist := openOrdersMap[activeOrder.OrderID]; exist {
			// no need to sync active order already in active orderbook, because we only need to know if it filled or not.
			delete(openOrdersMap, activeOrder.OrderID)
		} else {
			b.logger.Infof("found active order #%d is not in the open orders, updating...", activeOrder.OrderID)
			updatedOrder, err := b.SyncOrder(ctx, ex, activeOrder.OrderID, activeOrder.UUID, syncBefore)
			if err != nil {
				errs = multierr.Append(errs, err)
				continue
			}

			if updatedOrder != nil {
				updatedOrders = append(updatedOrders, *updatedOrder)
			}
		}
	}

	// update open orders not in active orders
	for _, openOrder := range openOrdersMap {
		b.logger.Infof("found open order #%d is not in active orderbook, updating...", openOrder.OrderID)
		// we don't add open orders into active orderbook if updated in 3 min, because we may receive message from websocket and add it twice.
		if openOrder.UpdateTime.After(syncBefore) {
			b.logger.Infof("open order #%d is updated in buffer duration (%s), skip updating...", openOrder.OrderID, bufferDuration.String())
			continue
		}

		b.Add(openOrder)
		updatedOrders = append(updatedOrders, openOrder)
	}

	return updatedOrders, errs
}

func (b *ActiveOrderBook) SyncOrder(
	ctx context.Context, ex types.Exchange, orderID uint64, orderUUID string, syncBefore time.Time,
) (*types.Order, error) {
	isMax := exchange.IsMaxExchange(ex)

	orderQueryService, ok := ex.(types.ExchangeOrderQueryService)
	if !ok {
		return nil, fmt.Errorf("exchange %s doesn't support ExchangeOrderQueryService", ex.Name())
	}

	updatedOrder, err := retry.QueryOrderUntilSuccessful(ctx, orderQueryService, types.OrderQuery{
		Symbol:    b.Symbol,
		OrderID:   strconv.FormatUint(orderID, 10),
		OrderUUID: orderUUID,
	})

	if err != nil {
		return nil, err
	}

	if updatedOrder == nil {
		return nil, fmt.Errorf("unexpected error, order object (%d) is a nil pointer, please check it", orderID)
	}

	// maxapi.OrderStateFinalizing does not mean the fee is calculated
	// we should only consider order state done for MAX
	if isMax && updatedOrder.OriginalStatus != string(maxapi.OrderStateDone) {
		return nil, nil
	}

	// should only trigger order update when the updated time is old enough
	if updatedOrder.UpdateTime.After(syncBefore) {
		return nil, nil
	}

	b.Update(*updatedOrder)
	return updatedOrder, nil
}

func categorizeOrderBySymbol(orders types.OrderSlice) map[string]types.OrderSlice {
	orderMap := map[string]types.OrderSlice{}

	for _, order := range orders {
		orderMap[order.Symbol] = append(orderMap[order.Symbol], order)
	}

	return orderMap
}

func filterCanceledOrders(orders types.OrderSlice) (ret types.OrderSlice) {
	for _, o := range orders {
		if o.Status == types.OrderStatusCanceled {
			continue
		}

		ret = append(ret, o)
	}

	return ret
}

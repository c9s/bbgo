package common

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange"
	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

func SyncActiveOrder(ctx context.Context, ex types.Exchange, orderQueryService types.ExchangeOrderQueryService, activeOrderBook *bbgo.ActiveOrderBook, orderID uint64, syncBefore time.Time) (isOrderUpdated bool, err error) {
	isMax := exchange.IsMaxExchange(ex)

	updatedOrder, err := retry.QueryOrderUntilSuccessful(ctx, orderQueryService, types.OrderQuery{
		Symbol:  activeOrderBook.Symbol,
		OrderID: strconv.FormatUint(orderID, 10),
	})

	if err != nil {
		return isOrderUpdated, err
	}

	if updatedOrder == nil {
		return isOrderUpdated, fmt.Errorf("unexpected error, order object (%d) is a nil pointer, please check common.SyncActiveOrder()", orderID)
	}

	// maxapi.OrderStateFinalizing does not mean the fee is calculated
	// we should only consider order state done for MAX
	if isMax && updatedOrder.OriginalStatus != string(maxapi.OrderStateDone) {
		return isOrderUpdated, nil
	}

	// should only trigger order update when the updated time is old enough
	isOrderUpdated = updatedOrder.UpdateTime.Before(syncBefore)
	if isOrderUpdated {
		activeOrderBook.Update(*updatedOrder)
	}

	return isOrderUpdated, nil
}

type SyncActiveOrdersOpts struct {
	Logger            *logrus.Entry
	Exchange          types.Exchange
	OrderQueryService types.ExchangeOrderQueryService
	ActiveOrderBook   *bbgo.ActiveOrderBook
	OpenOrders        []types.Order
}

func SyncActiveOrders(ctx context.Context, opts SyncActiveOrdersOpts) error {
	opts.Logger.Infof("[ActiveOrderRecover] syncActiveOrders")

	// only sync orders which is updated over 3 min, because we may receive from websocket and handle it twice
	syncBefore := time.Now().Add(-3 * time.Minute)

	activeOrders := opts.ActiveOrderBook.Orders()

	openOrdersMap := make(map[uint64]types.Order)
	for _, openOrder := range opts.OpenOrders {
		openOrdersMap[openOrder.OrderID] = openOrder
	}

	var errs error
	// update active orders not in open orders
	for _, activeOrder := range activeOrders {
		if _, exist := openOrdersMap[activeOrder.OrderID]; exist {
			// no need to sync active order already in active orderbook, because we only need to know if it filled or not.
			delete(openOrdersMap, activeOrder.OrderID)
		} else {
			opts.Logger.Infof("[ActiveOrderRecover] found active order #%d is not in the open orders, updating...", activeOrder.OrderID)

			isActiveOrderBookUpdated, err := SyncActiveOrder(ctx, opts.Exchange, opts.OrderQueryService, opts.ActiveOrderBook, activeOrder.OrderID, syncBefore)
			if err != nil {
				opts.Logger.WithError(err).Errorf("[ActiveOrderRecover] unable to query order #%d", activeOrder.OrderID)
				errs = multierr.Append(errs, err)
				continue
			}

			if !isActiveOrderBookUpdated {
				opts.Logger.Infof("[ActiveOrderRecover] active order #%d is updated in 3 min, skip updating...", activeOrder.OrderID)
			}
		}
	}

	// update open orders not in active orders
	for _, openOrder := range openOrdersMap {
		opts.Logger.Infof("found open order #%d is not in active orderbook, updating...", openOrder.OrderID)
		// we don't add open orders into active orderbook if updated in 3 min, because we may receive message from websocket and add it twice.
		if openOrder.UpdateTime.After(syncBefore) {
			opts.Logger.Infof("open order #%d is updated in 3 min, skip updating...", openOrder.OrderID)
			continue
		}

		opts.ActiveOrderBook.Add(openOrder)
	}

	return errs
}

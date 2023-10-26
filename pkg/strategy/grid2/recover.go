package grid2

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

/*
  	Background knowledge
  	1. active orderbook add orders only when receive new order event or call Add/Update method manually
  	2. active orderbook remove orders only when receive filled/cancelled event or call Remove/Update method manually
  	As a result
  	1. at the same twin-order-price, there is order in open orders but not in active orderbook
  		- not receive new order event
		  	=> add order into active orderbook
  	2. at the same twin-order-price, there is order in active orderbook but not in open orders
  		- not receive filled event
			=> query the filled order and call Update method
  	3. at the same twin-order-price, there is no order in open orders and no order in active orderbook
		- failed to create the order
			=> query the last order from trades to emit filled, and it will submit again
		- not receive new order event and the order filled before we find it.
			=> query the untracked order (also is the last order) from trades to emit filled and it will submit the reversed order
  	4. at the same twin-order-price, there are different orders in open orders and active orderbook
	  	- should not happen !!!
		  	=> log error
	5. at the same twin-order-price, there is the same order in open orders and active orderbook
		- normal case
			=> no need to do anything
	After killing pod, active orderbook must be empty. we can think it is the same as not receive new event.
	Process
	1. build twin orderbook with pins and open orders.
	2. build twin orderbook with pins and active orders.
	3. compare above twin orderbooks to add open orders into active orderbook and update active orders.
	4. run grid recover to make sure all the twin price has its order.
*/

func buildTwinOrderBook(pins []Pin, orders []types.Order) (*TwinOrderBook, error) {
	book := newTwinOrderBook(pins)

	for _, order := range orders {
		if err := book.AddOrder(order); err != nil {
			return nil, err
		}
	}

	return book, nil
}

func syncActiveOrder(ctx context.Context, activeOrderBook *bbgo.ActiveOrderBook, orderQueryService types.ExchangeOrderQueryService, orderID uint64) error {
	updatedOrder, err := retry.QueryOrderUntilSuccessful(ctx, orderQueryService, types.OrderQuery{
		Symbol:  activeOrderBook.Symbol,
		OrderID: strconv.FormatUint(orderID, 10),
	})

	if err != nil {
		return err
	}

	activeOrderBook.Update(*updatedOrder)

	return nil
}

func queryTradesToUpdateTwinOrderBook(
	ctx context.Context,
	symbol string,
	twinOrderBook *TwinOrderBook,
	queryTradesService types.ExchangeTradeHistoryService,
	queryOrderService types.ExchangeOrderQueryService,
	existedOrders *types.SyncOrderMap,
	since, until time.Time,
	logger func(format string, args ...interface{})) error {
	if twinOrderBook == nil {
		return fmt.Errorf("twin orderbook should not be nil, please check it")
	}

	var fromTradeID uint64 = 0
	var limit int64 = 1000
	for {
		trades, err := queryTradesService.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
			StartTime:   &since,
			EndTime:     &until,
			LastTradeID: fromTradeID,
			Limit:       limit,
		})

		if err != nil {
			return errors.Wrapf(err, "failed to query trades to recover the grid")
		}

		if logger != nil {
			logger("QueryTrades from %s <-> %s (from: %d) return %d trades", since, until, fromTradeID, len(trades))
		}

		for _, trade := range trades {
			if trade.Time.After(until) {
				return nil
			}

			if logger != nil {
				logger(trade.String())
			}

			if existedOrders.Exists(trade.OrderID) {
				// already queries, skip
				continue
			}
			order, err := retry.QueryOrderUntilSuccessful(ctx, queryOrderService, types.OrderQuery{
				Symbol:  trade.Symbol,
				OrderID: strconv.FormatUint(trade.OrderID, 10),
			})

			if err != nil {
				return errors.Wrapf(err, "failed to query order by trade (trade id: %d, order id: %d)", trade.ID, trade.OrderID)
			}

			if logger != nil {
				logger(order.String())
			}
			// avoid query this order again
			existedOrders.Add(*order)
			// add 1 to avoid duplicate
			fromTradeID = trade.ID + 1

			if err := twinOrderBook.AddOrder(*order); err != nil {
				return errors.Wrapf(err, "failed to add queried order into twin orderbook")
			}
		}

		// stop condition
		if int64(len(trades)) < limit {
			return nil
		}
	}
}

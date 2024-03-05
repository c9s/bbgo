package grid2

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange"
	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var syncWindow = -3 * time.Minute

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

func (s *Strategy) recover(ctx context.Context) error {
	historyService, implemented := s.session.Exchange.(types.ExchangeTradeHistoryService)
	// if the exchange doesn't support ExchangeTradeHistoryService, do not run recover
	if !implemented {
		s.logger.Warn("ExchangeTradeHistoryService is not implemented, can not recover grid")
		return nil
	}

	activeOrderBook := s.orderExecutor.ActiveMakerOrders()
	activeOrders := activeOrderBook.Orders()

	openOrders, err := retry.QueryOpenOrdersUntilSuccessfulLite(ctx, s.session.Exchange, s.Symbol)
	if err != nil {
		return err
	}

	// check if it's new strategy or need to recover
	if len(activeOrders) == 0 && len(openOrders) == 0 && s.GridProfitStats.InitialOrderID == 0 {
		// even though there is no open orders and initial orderID is 0
		// we still need to query trades to make sure if we need to recover or not
		trades, err := historyService.QueryTrades(ctx, s.Symbol, &types.TradeQueryOptions{
			// from 1, because some API will ignore 0 last trade id
			LastTradeID: 1,
			// if there is any trades, we need to recover.
			Limit: 1,
		})

		if err != nil {
			return errors.Wrapf(err, "unable to query trades when recovering")
		}

		if len(trades) == 0 {
			s.logger.Info("no open order, no active order, no trade, it's a new strategy so no need to recover")
			return nil
		}
	}

	s.logger.Info("start recovering")

	if s.getGrid() == nil {
		s.setGrid(s.newGrid())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pins := s.getGrid().Pins

	syncBefore := time.Now().Add(syncWindow)

	activeOrdersInTwinOrderBook, err := buildTwinOrderBook(pins, activeOrders)
	openOrdersInTwinOrderBook, err := buildTwinOrderBook(pins, openOrders)

	s.logger.Infof("[Recover] active orders' twin orderbook\n%s", activeOrdersInTwinOrderBook.String())
	s.logger.Infof("[Recover] open orders in twin orderbook\n%s", openOrdersInTwinOrderBook.String())

	// remove index 0, because twin orderbook's price is from the second one
	pins = pins[1:]
	var noTwinOrderPins []fixedpoint.Value

	for _, pin := range pins {
		v := fixedpoint.Value(pin)
		activeOrder := activeOrdersInTwinOrderBook.GetTwinOrder(v)
		openOrder := openOrdersInTwinOrderBook.GetTwinOrder(v)
		if activeOrder == nil || openOrder == nil {
			return fmt.Errorf("there is no any twin order at this pin, can not recover")
		}

		var activeOrderID uint64 = 0
		if activeOrder.Exist() {
			activeOrderID = activeOrder.GetOrder().OrderID
		}

		var openOrderID uint64 = 0
		if openOrder.Exist() {
			openOrderID = openOrder.GetOrder().OrderID
		}

		// case 3
		if activeOrderID == 0 && openOrderID == 0 {
			noTwinOrderPins = append(noTwinOrderPins, v)
			continue
		}

		// case 1
		if activeOrderID == 0 {
			order := openOrder.GetOrder()
			s.logger.Infof("[Recover] found open order #%d is not in the active orderbook, adding...", order.OrderID)
			activeOrderBook.Add(order)
			// also add open orders into active order's twin orderbook, we will use this active orderbook to recover empty price grid
			activeOrdersInTwinOrderBook.AddTwinOrder(v, openOrder)
			continue
		}

		// case 2
		if openOrderID == 0 {
			order := activeOrder.GetOrder()
			s.logger.Infof("[Recover] found active order #%d is not in the open orders, updating...", order.OrderID)
			isActiveOrderBookUpdated, err := syncActiveOrder(ctx, activeOrderBook, s.orderQueryService, order.OrderID, syncBefore)
			if err != nil {
				s.logger.WithError(err).Errorf("[Recover] unable to query order #%d", order.OrderID)
				continue
			}

			if !isActiveOrderBookUpdated {
				s.logger.Infof("[Recover] active order #%d is updated in 3 min, skip updating...", order.OrderID)
			}

			continue
		}

		// case 4
		if activeOrderID != openOrderID {
			return fmt.Errorf("there are two different orders in the same pin, can not recover")
		}

		// case 5
		// do nothing
	}

	s.logger.Infof("twin orderbook after adding open orders\n%s", activeOrdersInTwinOrderBook.String())

	if len(noTwinOrderPins) != 0 {
		if err := s.recoverEmptyGridOnTwinOrderBook(ctx, activeOrdersInTwinOrderBook, historyService, s.orderQueryService); err != nil {
			s.logger.WithError(err).Error("failed to recover empty grid")
			return err
		}

		s.logger.Infof("twin orderbook after recovering no twin order on grid\n%s", activeOrdersInTwinOrderBook.String())

		if activeOrdersInTwinOrderBook.EmptyTwinOrderSize() > 0 {
			return fmt.Errorf("there is still empty grid in twin orderbook")
		}

		for _, pin := range noTwinOrderPins {
			twinOrder := activeOrdersInTwinOrderBook.GetTwinOrder(pin)
			if twinOrder == nil {
				return fmt.Errorf("should not get nil twin order after recovering empty grid, check it")
			}

			if !twinOrder.Exist() {
				return fmt.Errorf("should not get empty twin order after recovering empty grid, check it")
			}

			activeOrderBook.EmitFilled(twinOrder.GetOrder())

			time.Sleep(100 * time.Millisecond)
		}
	}

	// TODO: do not emit ready here, emit ready only once when opening grid or recovering grid after worker stopped
	// s.EmitGridReady()

	time.Sleep(2 * time.Second)
	debugGrid(s.logger, s.grid, s.orderExecutor.ActiveMakerOrders())

	bbgo.Sync(ctx, s)

	return nil
}

func (s *Strategy) recoverEmptyGridOnTwinOrderBook(
	ctx context.Context,
	twinOrderBook *TwinOrderBook,
	queryTradesService types.ExchangeTradeHistoryService,
	queryOrderService types.ExchangeOrderQueryService,
) error {
	if twinOrderBook.EmptyTwinOrderSize() == 0 {
		s.logger.Info("no empty grid")
		return nil
	}

	existedOrders := twinOrderBook.SyncOrderMap()

	until := time.Now()
	since := until.Add(-1 * time.Hour)
	// hard limit for recover
	recoverSinceLimit := time.Date(2023, time.March, 10, 0, 0, 0, 0, time.UTC)

	if s.RecoverGridWithin != 0 && until.Add(-1*s.RecoverGridWithin).After(recoverSinceLimit) {
		recoverSinceLimit = until.Add(-1 * s.RecoverGridWithin)
	}

	for {
		if err := queryTradesToUpdateTwinOrderBook(ctx, s.Symbol, twinOrderBook, queryTradesService, queryOrderService, existedOrders, since, until, s.debugLog); err != nil {
			return errors.Wrapf(err, "failed to query trades to update twin orderbook")
		}

		until = since
		since = until.Add(-6 * time.Hour)

		if twinOrderBook.EmptyTwinOrderSize() == 0 {
			s.logger.Infof("stop querying trades because there is no empty twin order on twin orderbook")
			break
		}

		if s.GridProfitStats != nil && s.GridProfitStats.Since != nil && until.Before(*s.GridProfitStats.Since) {
			s.logger.Infof("stop querying trades because the time range is out of the strategy's since (%s)", *s.GridProfitStats.Since)
			break
		}

		if until.Before(recoverSinceLimit) {
			s.logger.Infof("stop querying trades because the time range is out of the limit (%s)", recoverSinceLimit)
			break
		}
	}

	return nil
}

func buildTwinOrderBook(pins []Pin, orders []types.Order) (*TwinOrderBook, error) {
	book := newTwinOrderBook(pins)

	for _, order := range orders {
		if err := book.AddOrder(order); err != nil {
			return nil, err
		}
	}

	return book, nil
}

func syncActiveOrder(
	ctx context.Context, activeOrderBook *bbgo.ActiveOrderBook, orderQueryService types.ExchangeOrderQueryService,
	orderID uint64, syncBefore time.Time,
) (isOrderUpdated bool, err error) {
	isMax := exchange.IsMaxExchange(orderQueryService)

	updatedOrder, err := retry.QueryOrderUntilSuccessful(ctx, orderQueryService, types.OrderQuery{
		Symbol:  activeOrderBook.Symbol,
		OrderID: strconv.FormatUint(orderID, 10),
	})

	if err != nil {
		return isOrderUpdated, err
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

func queryTradesToUpdateTwinOrderBook(
	ctx context.Context,
	symbol string,
	twinOrderBook *TwinOrderBook,
	queryTradesService types.ExchangeTradeHistoryService,
	queryOrderService types.ExchangeOrderQueryService,
	existedOrders *types.SyncOrderMap,
	since, until time.Time,
	logger func(format string, args ...interface{}),
) error {
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

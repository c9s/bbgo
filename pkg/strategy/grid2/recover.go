package grid2

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

func (s *Strategy) recoverByScanningTrades(ctx context.Context, session *bbgo.ExchangeSession) error {
	historyService, implemented := session.Exchange.(types.ExchangeTradeHistoryService)
	// if the exchange doesn't support ExchangeTradeHistoryService, do not run recover
	if !implemented {
		s.logger.Warn("ExchangeTradeHistoryService is not implemented, can not recover grid")
		return nil
	}

	s.logger.Infof("recover grid with group id: %d", s.OrderGroupID)

	openOrders, err := session.Exchange.QueryOpenOrders(ctx, s.Symbol)
	if err != nil {
		return errors.Wrapf(err, "unable to query open orders when recovering")
	}

	s.logger.Infof("found %d open orders left on the %s order book", len(openOrders), s.Symbol)

	// filter out the order with the group id belongs to this grid
	openOrdersOnGrid := filterOrdersOnGrid(s.OrderGroupID, openOrders)

	s.logger.Infof("found %d open orders belong to this grid on the %s order book", len(openOrdersOnGrid), s.Symbol)

	if s.GridProfitStats.InitialOrderID != 0 {
		s.logger.Info("InitialOrderID is already there, need to recover")
	} else if len(openOrdersOnGrid) != 0 {
		s.logger.Info("even though InitialOrderID is 0, there are open orders on grid so need to recover")
	} else {
		s.logger.Info("InitialOrderID is 0 and there is no open orders on grid, query trades to check it")
		// initial order id may be new strategy or lost data in redis, so we need to check trades + open orders
		// if there are open orders or trades, we need to recover
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
			s.logger.Info("0 trades found, it's a new strategy so no need to recover")
			return nil
		}
	}

	s.logger.Infof("start recovering")
	if err := s.recoverWithOpenOrdersByScanningTrades(ctx, historyService, openOrdersOnGrid); err != nil {
		return errors.Wrap(err, "grid recover error")
	}

	return nil
}

func (s *Strategy) recoverWithOpenOrdersByScanningTrades(ctx context.Context, historyService types.ExchangeTradeHistoryService, openOrdersOnGrid []types.Order) error {
	if s.orderQueryService == nil {
		return fmt.Errorf("orderQueryService is nil, it can't get orders by trade")
	}

	// set grid
	grid := s.newGrid()
	s.setGrid(grid)

	// add open orders to active order book
	s.addOrdersToActiveOrderBook(openOrdersOnGrid)

	expectedOrderNums := s.GridNum - 1
	openOrdersOnGridNums := int64(len(openOrdersOnGrid))
	s.debugLog("open orders nums: %d, expected nums: %d", openOrdersOnGridNums, expectedOrderNums)
	if expectedOrderNums == openOrdersOnGridNums {
		// no need to recover
		return nil
	} else if expectedOrderNums < openOrdersOnGridNums {
		return fmt.Errorf("amount of grid's open orders should not > amount of expected grid's orders")
	}

	// 1. build pin-order map
	pinOrdersOpen, err := s.buildPinOrderMap(grid.Pins, openOrdersOnGrid)
	if err != nil {
		return errors.Wrapf(err, "failed to build pin order map with open orders")
	}

	// 2. build the filled pin-order map by querying trades
	pinOrdersFilled, err := s.buildFilledPinOrderMapFromTrades(ctx, historyService, pinOrdersOpen)
	if err != nil {
		return errors.Wrapf(err, "failed to build filled pin order map")
	}

	// 3. get the filled orders from pin-order map
	filledOrders := pinOrdersFilled.AscendingOrders()
	numFilledOrders := len(filledOrders)
	if numFilledOrders == int(expectedOrderNums-openOrdersOnGridNums) {
		// nums of filled order is the same as Size - 1 - num(open orders)
	} else if numFilledOrders == int(expectedOrderNums-openOrdersOnGridNums+1) {
		filledOrders = filledOrders[1:]
	} else {
		return fmt.Errorf("not reasonable num of filled orders")
	}

	// 4. verify the grid
	if err := s.verifyFilledGrid(s.grid.Pins, pinOrdersOpen, filledOrders); err != nil {
		return errors.Wrapf(err, "verify grid with error")
	}

	s.debugOrders("emit filled orders", filledOrders)

	// 5. emit the filled orders
	activeOrderBook := s.orderExecutor.ActiveMakerOrders()
	for _, filledOrder := range filledOrders {
		activeOrderBook.EmitFilled(filledOrder)
	}

	// 6. emit grid ready
	s.EmitGridReady()

	// 7. debug and send metrics
	// wait for the reverse order to be placed
	time.Sleep(2 * time.Second)
	debugGrid(s.logger, grid, s.orderExecutor.ActiveMakerOrders())
	s.updateGridNumOfOrdersMetricsWithLock()
	s.updateOpenOrderPricesMetrics(s.orderExecutor.ActiveMakerOrders().Orders())

	return nil
}

func (s *Strategy) verifyFilledGrid(pins []Pin, pinOrders PinOrderMap, filledOrders []types.Order) error {
	s.debugLog("pins: %+v", pins)
	s.debugLog("open pin orders:\n%s", pinOrders.String())
	s.debugOrders("filled orders", filledOrders)

	for _, filledOrder := range filledOrders {
		price := filledOrder.Price
		if o, exist := pinOrders[price]; !exist {
			return fmt.Errorf("the price (%+v) is not in pins", price)
		} else if o.OrderID != 0 {
			return fmt.Errorf("there is already an order at this price (%+v)", price)
		} else {
			pinOrders[price] = filledOrder
		}
	}

	s.debugLog("filled pin orders:\n%+v", pinOrders.String())

	side := types.SideTypeBuy
	for _, pin := range pins {
		order, exist := pinOrders[fixedpoint.Value(pin)]
		if !exist {
			return fmt.Errorf("there is no order at price (%+v)", pin)
		}

		// if there is order with OrderID = 0, means we hit the empty pin
		// there must be only one empty pin in the grid
		// all orders below this pin need to be bid orders, above this pin need to be ask orders
		if order.OrderID == 0 {
			if side == types.SideTypeBuy {
				side = types.SideTypeSell
				continue
			}

			return fmt.Errorf("not only one empty order in this grid")
		}

		if order.Side != side {
			return fmt.Errorf("the side is wrong")
		}
	}

	if side != types.SideTypeSell {
		return fmt.Errorf("there is no empty pin in the grid")
	}

	return nil
}

// buildPinOrderMap build the pin-order map with grid and open orders.
// The keys of this map contains all required pins of this grid.
// If the Order of the pin is empty types.Order (OrderID == 0), it means there is no open orders at this pin.
func (s *Strategy) buildPinOrderMap(pins []Pin, openOrders []types.Order) (PinOrderMap, error) {
	pinOrderMap := make(PinOrderMap)

	for _, pin := range pins {
		pinOrderMap[fixedpoint.Value(pin)] = types.Order{}
	}

	for _, openOrder := range openOrders {
		pin := openOrder.Price
		v, exist := pinOrderMap[pin]
		if !exist {
			return nil, fmt.Errorf("the price of the order (id: %d) is not in pins", openOrder.OrderID)
		}

		if v.OrderID != 0 {
			return nil, fmt.Errorf("there are duplicated open orders at the same pin")
		}

		pinOrderMap[pin] = openOrder
	}

	return pinOrderMap, nil
}

// buildFilledPinOrderMapFromTrades will query the trades from last 24 hour and use them to build a pin order map
// It will skip the orders on pins at which open orders are already
func (s *Strategy) buildFilledPinOrderMapFromTrades(ctx context.Context, historyService types.ExchangeTradeHistoryService, pinOrdersOpen PinOrderMap) (PinOrderMap, error) {
	pinOrdersFilled := make(PinOrderMap)

	// existedOrders is used to avoid re-query the same orders
	existedOrders := pinOrdersOpen.SyncOrderMap()

	var limit int64 = 1000
	// get the filled orders when bbgo is down in order from trades
	// [NOTE] only retrieve from last 24 hours !!!
	var fromTradeID uint64 = 0
	for {
		trades, err := historyService.QueryTrades(ctx, s.Symbol, &types.TradeQueryOptions{
			LastTradeID: fromTradeID,
			Limit:       limit,
		})

		if err != nil {
			return nil, errors.Wrapf(err, "failed to query trades to recover the grid with open orders")
		}

		s.debugLog("QueryTrades return %d trades", len(trades))

		for _, trade := range trades {
			s.debugLog(trade.String())
			if existedOrders.Exists(trade.OrderID) {
				// already queries, skip
				continue
			}

			order, err := s.orderQueryService.QueryOrder(ctx, types.OrderQuery{
				OrderID: strconv.FormatUint(trade.OrderID, 10),
			})

			if err != nil {
				return nil, errors.Wrapf(err, "failed to query order by trade")
			}

			s.debugLog("%s (group_id: %d)", order.String(), order.GroupID)

			// avoid query this order again
			existedOrders.Add(*order)

			// add 1 to avoid duplicate
			fromTradeID = trade.ID + 1

			// this trade doesn't belong to this grid
			if order.GroupID != s.OrderGroupID {
				continue
			}

			// checked the trade's order is filled order
			pin := order.Price
			v, exist := pinOrdersOpen[pin]
			if !exist {
				return nil, fmt.Errorf("the price of the order with the same GroupID is not in pins")
			}

			// skip open orders on grid
			if v.OrderID != 0 {
				continue
			}

			// check the order's creation time
			if pinOrder, exist := pinOrdersFilled[pin]; exist && pinOrder.CreationTime.Time().After(order.CreationTime.Time()) {
				// do not replace the pin order if the order's creation time is not after pin order's creation time
				// this situation should not happen actually, because the trades is already sorted.
				s.logger.Infof("pinOrder's creation time (%s) should not be after order's creation time (%s)", pinOrder.CreationTime, order.CreationTime)
				continue
			}
			pinOrdersFilled[pin] = *order

			// wait 100 ms to avoid rate limit
			time.Sleep(100 * time.Millisecond)
		}

		// stop condition
		if int64(len(trades)) < limit {
			break
		}
	}

	return pinOrdersFilled, nil
}

func filterOrdersOnGrid(groupID uint32, orders []types.Order) []types.Order {
	var filteredOrders []types.Order
	for _, order := range orders {
		if order.GroupID != groupID {
			continue
		}

		filteredOrders = append(filteredOrders, order)
	}

	return filteredOrders
}

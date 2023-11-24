package grid2

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Strategy) recoverByScanningTrades(ctx context.Context, session *bbgo.ExchangeSession) error {
	defer func() {
		s.updateGridNumOfOrdersMetricsWithLock()
	}()
	isMax := isMaxExchange(session.Exchange)
	s.logger.Infof("isMax: %t", isMax)

	historyService, implemented := session.Exchange.(types.ExchangeTradeHistoryService)
	// if the exchange doesn't support ExchangeTradeHistoryService, do not run recover
	if !implemented {
		s.logger.Warn("ExchangeTradeHistoryService is not implemented, can not recover grid")
		return nil
	}

	openOrders, err := session.Exchange.QueryOpenOrders(ctx, s.Symbol)
	if err != nil {
		return errors.Wrapf(err, "unable to query open orders when recovering")
	}

	s.logger.Infof("found %d open orders left on the %s order book", len(openOrders), s.Symbol)

	if s.GridProfitStats.InitialOrderID != 0 {
		s.logger.Info("InitialOrderID is already there, need to recover")
	} else if len(openOrders) != 0 {
		s.logger.Info("even though InitialOrderID is 0, there are open orders so need to recover")
	} else {
		s.logger.Info("InitialOrderID is 0 and there is no open orders, query trades to check it")
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
	filledOrders, err := s.getFilledOrdersByScanningTrades(ctx, historyService, s.orderQueryService, openOrders)
	if err != nil {
		return errors.Wrap(err, "grid recover error")
	}
	s.debugOrders("emit filled orders", filledOrders)

	// add open orders into avtive maker orders
	s.addOrdersToActiveOrderBook(openOrders)

	// emit the filled orders
	activeOrderBook := s.orderExecutor.ActiveMakerOrders()
	for _, filledOrder := range filledOrders {
		if isMax && filledOrder.OriginalStatus != string(maxapi.OrderStateDone) {
			activeOrderBook.Add(filledOrder)
			continue
		}
		activeOrderBook.EmitFilled(filledOrder)
	}

	// emit ready after recover
	s.EmitGridReady()

	// debug and send metrics
	// wait for the reverse order to be placed
	time.Sleep(2 * time.Second)
	debugGrid(s.logger, s.grid, s.orderExecutor.ActiveMakerOrders())

	defer bbgo.Sync(ctx, s)

	if s.EnableProfitFixer {
		until := time.Now()
		since := until.Add(-7 * 24 * time.Hour)
		if s.FixProfitSince != nil {
			since = s.FixProfitSince.Time()
		}

		fixer := newProfitFixer(s.grid, s.Symbol, historyService)
		fixer.SetLogger(s.logger)

		// set initial order ID = 0 instead of s.GridProfitStats.InitialOrderID because the order ID could be incorrect
		if err := fixer.Fix(ctx, since, until, 0, s.GridProfitStats); err != nil {
			return err
		}

		s.logger.Infof("fixed profitStats: %#v", s.GridProfitStats)

		s.EmitGridProfit(s.GridProfitStats, nil)
	}

	return nil
}

func (s *Strategy) getFilledOrdersByScanningTrades(ctx context.Context, queryTradesService types.ExchangeTradeHistoryService, queryOrderService types.ExchangeOrderQueryService, openOrdersOnGrid []types.Order) ([]types.Order, error) {
	// set grid
	grid := s.newGrid()
	s.setGrid(grid)

	expectedNumOfOrders := s.GridNum - 1
	numGridOpenOrders := int64(len(openOrdersOnGrid))
	s.debugLog("open orders nums: %d, expected nums: %d", numGridOpenOrders, expectedNumOfOrders)
	if expectedNumOfOrders == numGridOpenOrders {
		// no need to recover, only need to add open orders back to active order book
		return nil, nil
	} else if expectedNumOfOrders < numGridOpenOrders {
		return nil, fmt.Errorf("amount of grid's open orders should not > amount of expected grid's orders")
	}

	// 1. build twin-order map
	twinOrdersOpen, err := s.buildTwinOrderMap(grid.Pins, openOrdersOnGrid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build pin order map with open orders")
	}

	// 2. build the filled twin-order map by querying trades
	expectedFilledNum := int(expectedNumOfOrders - numGridOpenOrders)
	twinOrdersFilled, err := s.buildFilledTwinOrderMapFromTrades(ctx, queryTradesService, queryOrderService, twinOrdersOpen, expectedFilledNum)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build filled pin order map")
	}

	// 3. get the filled orders from twin-order map
	filledOrders := twinOrdersFilled.AscendingOrders()

	// 4. verify the grid
	if err := s.verifyFilledTwinGrid(s.grid.Pins, twinOrdersOpen, filledOrders); err != nil {
		return nil, errors.Wrapf(err, "verify grid with error")
	}

	return filledOrders, nil
}

func (s *Strategy) verifyFilledTwinGrid(pins []Pin, twinOrders TwinOrderMap, filledOrders []types.Order) error {
	s.debugLog("verifying filled grid - pins: %+v", pins)
	s.debugOrders("verifying filled grid - filled orders", filledOrders)
	s.debugLog("verifying filled grid - open twin orders:\n%s", twinOrders.String())

	if err := s.addOrdersIntoTwinOrderMap(twinOrders, filledOrders); err != nil {
		return errors.Wrapf(err, "verifying filled grid error when add orders into twin order map")
	}

	s.debugLog("verifying filled grid - filled twin orders:\n%+v", twinOrders.String())

	for i, pin := range pins {
		// we use twinOrderMap to make sure there are no duplicated order at one grid, and we use the sell price as key so we skip the pins[0] which is only for buy price
		if i == 0 {
			continue
		}

		twin, exist := twinOrders[fixedpoint.Value(pin)]
		if !exist {
			return fmt.Errorf("there is no order at price (%+v)", pin)
		}

		if !twin.Exist() {
			return fmt.Errorf("all the price need a twin")
		}

		if !twin.IsValid() {
			return fmt.Errorf("all the twins need to be valid")
		}
	}

	return nil
}

// buildTwinOrderMap build the pin-order map with grid and open orders.
// The keys of this map contains all required pins of this grid.
// If the Order of the pin is empty types.Order (OrderID == 0), it means there is no open orders at this pin.
func (s *Strategy) buildTwinOrderMap(pins []Pin, openOrders []types.Order) (TwinOrderMap, error) {
	twinOrderMap := make(TwinOrderMap)

	for i, pin := range pins {
		// twin order map only use sell price as key, so skip 0
		if i == 0 {
			continue
		}

		twinOrderMap[fixedpoint.Value(pin)] = TwinOrder{}
	}

	for _, openOrder := range openOrders {
		twinKey, err := findTwinOrderMapKey(s.grid, openOrder)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to build twin order map")
		}

		twinOrder, exist := twinOrderMap[twinKey]
		if !exist {
			return nil, fmt.Errorf("the price of the openOrder (id: %d) is not in pins", openOrder.OrderID)
		}

		if twinOrder.Exist() {
			return nil, fmt.Errorf("there are multiple order in a twin")
		}

		twinOrder.SetOrder(openOrder)
		twinOrderMap[twinKey] = twinOrder
	}

	return twinOrderMap, nil
}

// buildFilledTwinOrderMapFromTrades will query the trades from last 24 hour and use them to build a pin order map
// It will skip the orders on pins at which open orders are already
func (s *Strategy) buildFilledTwinOrderMapFromTrades(ctx context.Context, queryTradesService types.ExchangeTradeHistoryService, queryOrderService types.ExchangeOrderQueryService, twinOrdersOpen TwinOrderMap, expectedFillNum int) (TwinOrderMap, error) {
	twinOrdersFilled := make(TwinOrderMap)

	// existedOrders is used to avoid re-query the same orders
	existedOrders := twinOrdersOpen.SyncOrderMap()

	// get the filled orders when bbgo is down in order from trades
	until := time.Now()
	// the first query only query the last 1 hour, because mostly shutdown and recovery happens within 1 hour
	since := until.Add(-1 * time.Hour)
	// hard limit for recover
	recoverSinceLimit := time.Date(2023, time.March, 10, 0, 0, 0, 0, time.UTC)

	if s.RecoverGridWithin != 0 && until.Add(-1*s.RecoverGridWithin).After(recoverSinceLimit) {
		recoverSinceLimit = until.Add(-1 * s.RecoverGridWithin)
	}

	for {
		if err := s.queryTradesToUpdateTwinOrdersMap(ctx, queryTradesService, queryOrderService, twinOrdersOpen, twinOrdersFilled, existedOrders, since, until); err != nil {
			return nil, errors.Wrapf(err, "failed to query trades to update twin orders map")
		}

		until = since
		since = until.Add(-6 * time.Hour)

		if len(twinOrdersFilled) >= expectedFillNum {
			s.logger.Infof("stop querying trades because twin orders filled (%d) >= expected filled nums (%d)", len(twinOrdersFilled), expectedFillNum)
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

	return twinOrdersFilled, nil
}

func (s *Strategy) queryTradesToUpdateTwinOrdersMap(ctx context.Context, queryTradesService types.ExchangeTradeHistoryService, queryOrderService types.ExchangeOrderQueryService, twinOrdersOpen, twinOrdersFilled TwinOrderMap, existedOrders *types.SyncOrderMap, since, until time.Time) error {
	var fromTradeID uint64 = 0
	var limit int64 = 1000
	for {
		trades, err := queryTradesService.QueryTrades(ctx, s.Symbol, &types.TradeQueryOptions{
			StartTime:   &since,
			EndTime:     &until,
			LastTradeID: fromTradeID,
			Limit:       limit,
		})

		if err != nil {
			return errors.Wrapf(err, "failed to query trades to recover the grid with open orders")
		}

		s.debugLog("QueryTrades from %s <-> %s (from: %d) return %d trades", since, until, fromTradeID, len(trades))

		for _, trade := range trades {
			if trade.Time.After(until) {
				return nil
			}

			s.debugLog(trade.String())

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

			s.debugLog(order.String())
			// avoid query this order again
			existedOrders.Add(*order)
			// add 1 to avoid duplicate
			fromTradeID = trade.ID + 1

			twinOrderKey, err := findTwinOrderMapKey(s.grid, *order)
			if err != nil {
				return errors.Wrapf(err, "failed to find grid order map's key when recover")
			}

			twinOrderOpen, exist := twinOrdersOpen[twinOrderKey]
			if !exist {
				return fmt.Errorf("the price of the order with the same GroupID is not in pins")
			}

			if twinOrderOpen.Exist() {
				continue
			}

			if twinOrder, exist := twinOrdersFilled[twinOrderKey]; exist {
				to := twinOrder.GetOrder()
				if to.UpdateTime.Time().After(order.UpdateTime.Time()) {
					s.logger.Infof("twinOrder's update time (%s) should not be after order's update time (%s)", to.UpdateTime, order.UpdateTime)
					continue
				}
			}

			twinOrder := TwinOrder{}
			twinOrder.SetOrder(*order)
			twinOrdersFilled[twinOrderKey] = twinOrder
		}

		// stop condition
		if int64(len(trades)) < limit {
			return nil
		}
	}
}

func (s *Strategy) addOrdersIntoTwinOrderMap(twinOrders TwinOrderMap, orders []types.Order) error {
	for _, order := range orders {
		k, err := findTwinOrderMapKey(s.grid, order)
		if err != nil {
			return errors.Wrap(err, "failed to add orders into twin order map")
		}

		if v, exist := twinOrders[k]; !exist {
			return fmt.Errorf("the price (%+v) is not in pins", k)
		} else if v.Exist() {
			return fmt.Errorf("there is already a twin order at this price (%+v)", k)
		} else {
			twin := TwinOrder{}
			twin.SetOrder(order)
			twinOrders[k] = twin
		}
	}

	return nil
}

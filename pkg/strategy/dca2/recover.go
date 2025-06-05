package dca2

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/strategy/dca2/statemachine"
	"github.com/c9s/bbgo/pkg/types"
)

type descendingClosedOrderQueryService interface {
	QueryClosedOrdersDesc(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error)
}

func (s *Strategy) recover(ctx context.Context) error {
	s.logger.Info("[DCA] recover")
	currentRound, err := s.collector.CollectCurrentRound(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}
	debugRoundOrders(s.logger, "current", currentRound)

	// recover profit stats
	if s.DisableProfitStatsRecover {
		s.logger.Info("disableProfitStatsRecover is set, skip profit stats recovery")
	} else {
		if err := recoverProfitStats(ctx, s); err != nil {
			return err
		}
		s.logger.Info("recover profit stats DONE")
	}

	// recover position
	if s.DisablePositionRecover {
		s.logger.Info("disablePositionRecover is set, skip position recovery")
	} else {
		if err := recoverPosition(ctx, s.Position, currentRound, s.collector.queryService); err != nil {
			return err
		}
		s.logger.Info("recover position DONE")
	}

	// recover startTimeOfNextRound
	startTimeOfNextRound := recoverStartTimeOfNextRound(ctx, currentRound, s.CoolDownInterval)
	s.startTimeOfNextRound = startTimeOfNextRound

	// recover state
	state, err := recoverState(currentRound, s.OrderExecutor)
	if err != nil {
		return err
	}
	s.updateState(state)
	s.logger.Info("recover stats DONE")

	return nil
}

// recover state
func recoverState(currentRound Round, orderExecutor *bbgo.GeneralOrderExecutor) (statemachine.State, error) {
	// no open-position orders and no take-profit orders means this is the whole new strategy
	if len(currentRound.OpenPositionOrders) == 0 && len(currentRound.TakeProfitOrders) == 0 {
		return IdleWaiting, nil
	}

	// it should not happen
	if len(currentRound.OpenPositionOrders) == 0 && len(currentRound.TakeProfitOrders) > 0 {
		return None, fmt.Errorf("there is no open-position orders but there are take-profit orders. it should not happen, please check it")
	}

	activeOrderBook := orderExecutor.ActiveMakerOrders()
	orderStore := orderExecutor.OrderStore()

	if len(currentRound.TakeProfitOrders) > 0 {
		return recoverStateIfAtTakeProfitStage(currentRound.TakeProfitOrders, activeOrderBook, orderStore)
	}

	return recoverStateIfAtOpenPositionStage(currentRound.OpenPositionOrders, activeOrderBook, orderStore)
}

// recoverStateIfAtTakeProfitStage will recover the state if the strategy is stopped at take-profit stage
func recoverStateIfAtTakeProfitStage(orders types.OrderSlice, activeOrderBook *bbgo.ActiveOrderBook, orderStore *core.OrderStore) (statemachine.State, error) {
	// because we may manually recover the strategy by cancelling the orders and placing the orders again, so we don't need to consider the cancelled orders
	openedOrders, cancelledOrders, filledOrders, unexpectedOrders := orders.ClassifyByStatus()

	if len(unexpectedOrders) > 0 {
		return None, fmt.Errorf("there is unexpected status in orders %+v at take-profit stage recovery", unexpectedOrders)
	}

	// add opened order into order store
	for _, order := range openedOrders {
		activeOrderBook.Add(order)
		orderStore.Add(order)
	}

	// no open orders and there are filled orders -> means this round is finished, will start a new round
	if len(filledOrders) > 0 && len(openedOrders) == 0 {
		return IdleWaiting, nil
	}

	// there are open orders -> means this round is not finished, we still at TakeProfitReady state
	if len(openedOrders) > 0 {
		return TakeProfitReady, nil
	}

	// only len(openedOrders) == 0 and len(filledOrders) == 0 len(cancelledOrders) > 0 will reach this line
	return None, fmt.Errorf("the classify orders count is not expected (opened: %d, cancelled: %d, filled: %d) at take-profit stage recovery", len(openedOrders), len(cancelledOrders), len(filledOrders))
}

// recoverStateIfAtOpenPositionStage will recover the state if the strategy is stopped at open-position stage
func recoverStateIfAtOpenPositionStage(orders types.OrderSlice, activeOrderBook *bbgo.ActiveOrderBook, orderStore *core.OrderStore) (statemachine.State, error) {
	openedOrders, cancelledOrders, filledOrders, unexpectedOrders := orders.ClassifyByStatus()
	if len(unexpectedOrders) > 0 {
		return None, fmt.Errorf("there is unexpected status of orders %+v at open-position stage recovery", unexpectedOrders)
	}

	// add opened order into order store
	for _, order := range openedOrders {
		activeOrderBook.Add(order)
		orderStore.Add(order)
	}

	// if there is any cancelled order, it means the open-position stage is finished
	if len(cancelledOrders) > 0 {
		return OpenPositionFinished, nil
	}

	// if there is no open order, it means all the orders were filled so this open-position stage is finished
	if len(openedOrders) == 0 {
		return OpenPositionFinished, nil
	}

	// if there is no filled order, it means we need to wait for at least one order filled
	if len(filledOrders) == 0 {
		return OpenPositionReady, nil
	}

	// if there is at least one filled order and still open orders, it means we are ready to take profit if we hit the take-profit price
	return OpenPositionOrderFilled, nil
}

func recoverPosition(ctx context.Context, position *types.Position, currentRound Round, queryService types.ExchangeOrderQueryService) error {
	if position == nil {
		return fmt.Errorf("position is nil, please check it")
	}

	// reset position to recover
	position.Reset()

	var positionOrders []types.Order

	var filledCnt int64
	for _, order := range currentRound.TakeProfitOrders {
		if !types.IsActiveOrder(order) {
			filledCnt++
		}
		positionOrders = append(positionOrders, order)
	}

	// all take-profit orders are filled
	if len(currentRound.TakeProfitOrders) > 0 && filledCnt == int64(len(currentRound.TakeProfitOrders)) {
		return nil
	}

	for _, order := range currentRound.OpenPositionOrders {
		// no executed quantity order, no need to get trades
		if order.ExecutedQuantity.IsZero() {
			continue
		}

		positionOrders = append(positionOrders, order)
	}

	for _, positionOrder := range positionOrders {
		trades, err := retry.QueryOrderTradesUntilSuccessful(ctx, queryService, types.OrderQuery{
			Symbol:  position.Symbol,
			OrderID: strconv.FormatUint(positionOrder.OrderID, 10),
		})

		if err != nil {
			return errors.Wrapf(err, "failed to get order (%d) trades", positionOrder.OrderID)
		}
		position.AddTrades(trades)
	}

	return nil
}

func recoverProfitStats(ctx context.Context, strategy *Strategy) error {
	if strategy.ProfitStats == nil {
		return fmt.Errorf("profit stats is nil, please check it")
	}

	_, err := strategy.UpdateProfitStats(ctx)
	return err
}

func recoverStartTimeOfNextRound(ctx context.Context, currentRound Round, coolDownInterval types.Duration) time.Time {
	var startTimeOfNextRound time.Time

	for _, order := range currentRound.TakeProfitOrders {
		if t := order.UpdateTime.Time().Add(coolDownInterval.Duration()); t.After(startTimeOfNextRound) {
			startTimeOfNextRound = t
		}
	}

	return startTimeOfNextRound
}

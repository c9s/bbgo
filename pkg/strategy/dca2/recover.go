package dca2

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

var recoverSinceLimit = time.Date(2024, time.January, 29, 12, 0, 0, 0, time.Local)

type descendingClosedOrderQueryService interface {
	QueryClosedOrdersDesc(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error)
}

func (s *Strategy) recover(ctx context.Context) error {
	s.logger.Info("[DCA] recover")
	currentRound, err := s.collector.CollectCurrentRound(ctx)
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
	state, err := recoverState(ctx, int(s.MaxOrderCount), currentRound, s.OrderExecutor)
	if err != nil {
		return err
	}
	s.updateState(state)
	s.logger.Info("recover stats DONE")

	return nil
}

// recover state
func recoverState(ctx context.Context, maxOrderCount int, currentRound Round, orderExecutor *bbgo.GeneralOrderExecutor) (State, error) {
	activeOrderBook := orderExecutor.ActiveMakerOrders()
	orderStore := orderExecutor.OrderStore()

	// dca stop at take-profit order stage
	if currentRound.TakeProfitOrder.OrderID != 0 {
		// the number of open-positions orders may not be equal to maxOrderCount, because the notional may not enough to open maxOrderCount orders
		if len(currentRound.OpenPositionOrders) > maxOrderCount {
			return None, fmt.Errorf("there is take-profit order but the number of open-position orders (%d) is greater than maxOrderCount(%d). Please check it", len(currentRound.OpenPositionOrders), maxOrderCount)
		}

		takeProfitOrder := currentRound.TakeProfitOrder
		if takeProfitOrder.Status == types.OrderStatusFilled {
			return WaitToOpenPosition, nil
		} else if types.IsActiveOrder(takeProfitOrder) {
			activeOrderBook.Add(takeProfitOrder)
			orderStore.Add(takeProfitOrder)
			return TakeProfitReady, nil
		} else {
			return None, fmt.Errorf("the status of take-profit order is %s. Please check it", takeProfitOrder.Status)
		}
	}

	// dca stop at no take-profit order stage
	openPositionOrders := currentRound.OpenPositionOrders
	numOpenPositionOrders := len(openPositionOrders)

	// new strategy
	if len(openPositionOrders) == 0 {
		return WaitToOpenPosition, nil
	}

	// should not happen
	if numOpenPositionOrders > maxOrderCount {
		return None, fmt.Errorf("the number of open-position orders (%d) is > max order number", numOpenPositionOrders)
	}

	// collect open-position orders' status
	var openedCnt, filledCnt, cancelledCnt int64
	for _, order := range currentRound.OpenPositionOrders {
		switch order.Status {
		case types.OrderStatusNew, types.OrderStatusPartiallyFilled:
			activeOrderBook.Add(order)
			orderStore.Add(order)
			openedCnt++
		case types.OrderStatusFilled:
			filledCnt++
		case types.OrderStatusCanceled:
			cancelledCnt++
		default:
			return None, fmt.Errorf("there is unexpected status %s of order %s", order.Status, order)
		}
	}

	// all open-position orders are still not filled -> OpenPositionReady
	if filledCnt == 0 && cancelledCnt == 0 {
		return OpenPositionReady, nil
	}

	// there are at least one open-position orders filled
	if filledCnt > 0 && cancelledCnt == 0 {
		if openedCnt > 0 {
			return OpenPositionOrderFilled, nil
		} else {
			// all open-position orders filled, change to cancelling and place the take-profit order
			return OpenPositionOrdersCancelling, nil
		}
	}

	// there are at last one open-position orders cancelled ->
	if cancelledCnt > 0 {
		return OpenPositionOrdersCancelling, nil
	}

	return None, fmt.Errorf("unexpected order status combination (opened, filled, cancelled) = (%d, %d, %d)", openedCnt, filledCnt, cancelledCnt)
}

func recoverPosition(ctx context.Context, position *types.Position, currentRound Round, queryService types.ExchangeOrderQueryService) error {
	if position == nil {
		return fmt.Errorf("position is nil, please check it")
	}

	var positionOrders []types.Order
	position.Reset()
	if currentRound.TakeProfitOrder.OrderID != 0 {
		if !types.IsActiveOrder(currentRound.TakeProfitOrder) {
			return nil
		}

		positionOrders = append(positionOrders, currentRound.TakeProfitOrder)
	}

	for _, order := range currentRound.OpenPositionOrders {
		// no executed quantity order, no need to get trades
		if order.ExecutedQuantity.IsZero() {
			continue
		}

		positionOrders = append(positionOrders, order)
	}

	for _, positionOrder := range positionOrders {
		trades, err := queryService.QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  position.Symbol,
			OrderID: strconv.FormatUint(positionOrder.OrderID, 10),
		})

		if err != nil {
			return fmt.Errorf("failed to get trades of order (%d)", positionOrder.OrderID)
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
	if currentRound.TakeProfitOrder.OrderID != 0 && currentRound.TakeProfitOrder.Status == types.OrderStatusFilled {
		return currentRound.TakeProfitOrder.UpdateTime.Time().Add(coolDownInterval.Duration())
	}

	return time.Time{}
}

type Round struct {
	OpenPositionOrders []types.Order
	TakeProfitOrder    types.Order
}

func getCurrentRoundOrders(openOrders, closedOrders []types.Order, groupID uint32) (Round, error) {
	openPositionSide := types.SideTypeBuy
	takeProfitSide := types.SideTypeSell

	var allOrders []types.Order
	allOrders = append(allOrders, openOrders...)
	allOrders = append(allOrders, closedOrders...)

	types.SortOrdersDescending(allOrders)

	var currentRound Round
	lastSide := takeProfitSide
	for _, order := range allOrders {
		// group id filter is used for debug when local running
		if order.GroupID != groupID {
			continue
		}

		if order.Side == takeProfitSide && lastSide == openPositionSide {
			break
		}

		switch order.Side {
		case openPositionSide:
			currentRound.OpenPositionOrders = append(currentRound.OpenPositionOrders, order)
		case takeProfitSide:
			if currentRound.TakeProfitOrder.OrderID != 0 {
				return currentRound, fmt.Errorf("there are two take-profit orders in one round, please check it")
			}
			currentRound.TakeProfitOrder = order
		default:
		}

		lastSide = order.Side
	}

	return currentRound, nil
}

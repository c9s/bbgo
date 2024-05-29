package dca2

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
	if len(currentRound.TakeProfitOrders) > 0 {
		openedOrders, cancelledOrders, filledOrders, unexpectedOrders := classifyOrders(currentRound.TakeProfitOrders)

		if len(unexpectedOrders) > 0 {
			return None, fmt.Errorf("there is unexpected status in orders %+v", unexpectedOrders)
		}

		if len(filledOrders) > 0 && len(openedOrders) == 0 {
			return WaitToOpenPosition, nil
		}

		if len(filledOrders) == 0 && len(openedOrders) > 0 {
			// add opened order into order store
			for _, order := range openedOrders {
				activeOrderBook.Add(order)
				orderStore.Add(order)
			}
			return TakeProfitReady, nil
		}

		return None, fmt.Errorf("the classify orders count is not expected (opened: %d, cancelled: %d, filled: %d)", len(openedOrders), len(cancelledOrders), len(filledOrders))
	}

	// dca stop at no take-profit order stage
	openPositionOrders := currentRound.OpenPositionOrders

	// new strategy
	if len(openPositionOrders) == 0 {
		return WaitToOpenPosition, nil
	}

	// collect open-position orders' status
	openedOrders, cancelledOrders, filledOrders, unexpectedOrders := classifyOrders(currentRound.OpenPositionOrders)
	if len(unexpectedOrders) > 0 {
		return None, fmt.Errorf("there is unexpected status of orders %+v", unexpectedOrders)
	}
	for _, order := range openedOrders {
		activeOrderBook.Add(order)
		orderStore.Add(order)
	}

	// no order is filled -> OpenPositionReady
	if len(filledOrders) == 0 {
		return OpenPositionReady, nil
	}

	// there are at least one open-position orders filled
	if len(cancelledOrders) == 0 {
		if len(openedOrders) > 0 {
			return OpenPositionOrderFilled, nil
		} else {
			// all open-position orders filled, change to cancelling and place the take-profit order
			return OpenPositionOrdersCancelling, nil
		}
	}

	// there are at last one open-position orders cancelled and at least one filled order -> open position order cancelling
	return OpenPositionOrdersCancelling, nil
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

func classifyOrders(orders []types.Order) (opened, cancelled, filled, unexpected []types.Order) {
	for _, order := range orders {
		switch order.Status {
		case types.OrderStatusNew, types.OrderStatusPartiallyFilled:
			opened = append(opened, order)
		case types.OrderStatusFilled:
			filled = append(filled, order)
		case types.OrderStatusCanceled:
			cancelled = append(cancelled, order)
		default:
			unexpected = append(unexpected, order)
		}
	}

	return opened, cancelled, filled, unexpected
}

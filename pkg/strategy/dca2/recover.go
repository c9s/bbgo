package dca2

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type descendingClosedOrderQueryService interface {
	QueryClosedOrdersDesc(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error)
}

type RecoverApiQueryService interface {
	types.ExchangeOrderQueryService
	types.ExchangeTradeService
	descendingClosedOrderQueryService
}

func (s *Strategy) recover(ctx context.Context) error {
	s.logger.Info("[DCA] recover")
	queryService, ok := s.Session.Exchange.(RecoverApiQueryService)
	if !ok {
		return fmt.Errorf("[DCA] exchange %s doesn't support queryAPI interface", s.Session.ExchangeName)
	}

	openOrders, err := queryService.QueryOpenOrders(ctx, s.Symbol)
	if err != nil {
		return err
	}

	closedOrders, err := queryService.QueryClosedOrdersDesc(ctx, s.Symbol, time.Date(2024, time.January, 1, 0, 0, 0, 0, time.Local), time.Now(), 0)
	if err != nil {
		return err
	}

	currentRound, err := getCurrentRoundOrders(openOrders, closedOrders, s.OrderGroupID)
	if err != nil {
		return err
	}
	debugRoundOrders(s.logger, "current", currentRound)

	// recover state
	state, err := recoverState(ctx, s.Symbol, int(s.MaxOrderCount), openOrders, currentRound, s.OrderExecutor.ActiveMakerOrders(), s.OrderExecutor.OrderStore(), s.OrderGroupID)
	if err != nil {
		return err
	}

	// recover position
	if err := recoverPosition(ctx, s.Position, queryService, currentRound); err != nil {
		return err
	}

	// recover profit stats
	recoverProfitStats(ctx, s)

	// recover startTimeOfNextRound
	startTimeOfNextRound := recoverStartTimeOfNextRound(ctx, currentRound, s.CoolDownInterval)

	s.state = state
	s.startTimeOfNextRound = startTimeOfNextRound

	return nil
}

// recover state
func recoverState(ctx context.Context, symbol string, maxOrderCount int, openOrders []types.Order, currentRound Round, activeOrderBook *bbgo.ActiveOrderBook, orderStore *core.OrderStore, groupID uint32) (State, error) {
	if len(currentRound.OpenPositionOrders) == 0 {
		// new strategy
		return WaitToOpenPosition, nil
	}

	numOpenOrders := len(openOrders)
	// dca stop at take profit order stage
	if currentRound.TakeProfitOrder.OrderID != 0 {
		if numOpenOrders == 0 {
			// current round's take-profit order filled, wait to open next round
			return WaitToOpenPosition, nil
		}

		// check the open orders is take profit order or not
		if numOpenOrders == 1 {
			if openOrders[0].OrderID == currentRound.TakeProfitOrder.OrderID {
				activeOrderBook.Add(openOrders[0])
				// current round's take-profit order still opened, wait to fill
				return TakeProfitReady, nil
			} else {
				return None, fmt.Errorf("stop at taking profit stage, but the open order's OrderID is not the take-profit order's OrderID")
			}
		}

		return None, fmt.Errorf("stop at taking profit stage, but the number of open orders is > 1")
	}

	numOpenPositionOrders := len(currentRound.OpenPositionOrders)
	if numOpenPositionOrders > maxOrderCount {
		return None, fmt.Errorf("the number of open-position orders is > max order number")
	} else if numOpenPositionOrders < maxOrderCount {
		// The number of open-position orders should be the same as maxOrderCount
		// If not, it may be the following possible cause
		// 1. This strategy at position opening, so it may not place all orders we want successfully
		// 2. There are some errors when placing open-position orders. e.g. cannot lock fund.....
		return None, fmt.Errorf("the number of open-position orders is < max order number")
	}

	if numOpenOrders > numOpenPositionOrders {
		return None, fmt.Errorf("the number of open orders is > the number of open-position orders")
	}

	if numOpenOrders == numOpenPositionOrders {
		activeOrderBook.Add(openOrders...)
		orderStore.Add(openOrders...)
		return OpenPositionReady, nil
	}

	var openedCnt, filledCnt, cancelledCnt int64
	for _, order := range currentRound.OpenPositionOrders {
		switch order.Status {
		case types.OrderStatusNew, types.OrderStatusPartiallyFilled:
			openedCnt++
		case types.OrderStatusFilled:
			filledCnt++
		case types.OrderStatusCanceled:
			cancelledCnt++
		default:
			return None, fmt.Errorf("there is unexpected status %s of order %s", order.Status, order)
		}
	}

	if filledCnt > 0 && cancelledCnt == 0 {
		activeOrderBook.Add(openOrders...)
		orderStore.Add(openOrders...)
		return OpenPositionOrderFilled, nil
	}

	if openedCnt > 0 && filledCnt > 0 && cancelledCnt > 0 {
		return OpenPositionOrdersCancelling, nil
	}

	if openedCnt == 0 && filledCnt > 0 && cancelledCnt > 0 {
		return OpenPositionOrdersCancelled, nil
	}

	return None, fmt.Errorf("unexpected order status combination")
}

func recoverPosition(ctx context.Context, position *types.Position, queryService RecoverApiQueryService, currentRound Round) error {
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

	strategy.CalculateProfitOfCurrentRound(ctx)

	return nil
}

func recoverQuoteInvestment(currentRound Round) fixedpoint.Value {
	if len(currentRound.OpenPositionOrders) == 0 {
		return fixedpoint.Zero
	}

	total := fixedpoint.Zero
	for _, order := range currentRound.OpenPositionOrders {
		total = total.Add(order.Quantity.Mul(order.Price))
	}

	if currentRound.TakeProfitOrder.OrderID != 0 && currentRound.TakeProfitOrder.Status == types.OrderStatusFilled {
		total = total.Add(currentRound.TakeProfitOrder.Quantity.Mul(currentRound.TakeProfitOrder.Price))
		for _, order := range currentRound.OpenPositionOrders {
			total = total.Sub(order.ExecutedQuantity.Mul(order.Price))
		}
	}

	return total
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

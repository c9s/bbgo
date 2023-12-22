package dca2

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type queryAPI interface {
	QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error)
	QueryClosedOrdersDesc(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error)
	QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error)
}

func (s *Strategy) recover(ctx context.Context) error {
	s.logger.Info("[DCA] recover")
	queryService, ok := s.Session.Exchange.(queryAPI)
	if !ok {
		return fmt.Errorf("[DCA] exchange %s doesn't support queryAPI interface", s.Session.ExchangeName)
	}

	openOrders, err := queryService.QueryOpenOrders(ctx, s.Symbol)
	if err != nil {
		return err
	}

	closedOrders, err := queryService.QueryClosedOrdersDesc(ctx, s.Symbol, time.Time{}, time.Now(), 0)
	if err != nil {
		return err
	}

	currentRound, err := getCurrentRoundOrders(s.Short, openOrders, closedOrders, s.OrderGroupID)
	if err != nil {
		return err
	}
	debugRoundOrders(s.logger, "current", currentRound)

	// recover state
	state, err := recoverState(ctx, s.Symbol, s.Short, int(s.MaxOrderNum), openOrders, currentRound, s.OrderExecutor.ActiveMakerOrders(), s.OrderExecutor.OrderStore(), s.OrderGroupID)
	if err != nil {
		return err
	}

	// recover position
	if err := recoverPosition(ctx, s.Position, queryService, currentRound); err != nil {
		return err
	}

	// recover budget
	budget := recoverBudget(currentRound)

	// recover startTimeOfNextRound
	startTimeOfNextRound := recoverStartTimeOfNextRound(ctx, currentRound, s.CoolDownInterval)

	s.state = state
	if !budget.IsZero() {
		s.Budget = budget
	}
	s.startTimeOfNextRound = startTimeOfNextRound

	return nil
}

// recover state
func recoverState(ctx context.Context, symbol string, short bool, maxOrderNum int, openOrders []types.Order, currentRound Round, activeOrderBook *bbgo.ActiveOrderBook, orderStore *core.OrderStore, groupID uint32) (State, error) {
	numOpenOrders := len(openOrders)
	// dca stop at take profit order stage
	if currentRound.TakeProfitOrder.OrderID != 0 {
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

		if numOpenOrders == 0 {
			// current round's take-profit order filled, wait to open next round
			return WaitToOpenPosition, nil
		}

		return None, fmt.Errorf("stop at taking profit stage, but the number of open orders is > 1")
	}

	if len(currentRound.OpenPositionOrders) == 0 {
		// new strategy
		return WaitToOpenPosition, nil
	}

	numOpenPositionOrders := len(currentRound.OpenPositionOrders)
	if numOpenPositionOrders > maxOrderNum {
		return None, fmt.Errorf("the number of open-position orders is > max order number")
	} else if numOpenPositionOrders < maxOrderNum {
		// failed to place some orders at open position stage
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

func recoverPosition(ctx context.Context, position *types.Position, queryService queryAPI, currentRound Round) error {
	if position == nil {
		return nil
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

func recoverBudget(currentRound Round) fixedpoint.Value {
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

func getCurrentRoundOrders(short bool, openOrders, closedOrders []types.Order, groupID uint32) (Round, error) {
	openPositionSide := types.SideTypeBuy
	takeProfitSide := types.SideTypeSell

	if short {
		openPositionSide = types.SideTypeSell
		takeProfitSide = types.SideTypeBuy
	}

	var allOrders []types.Order
	allOrders = append(allOrders, openOrders...)
	allOrders = append(allOrders, closedOrders...)

	sort.Slice(allOrders, func(i, j int) bool {
		return allOrders[i].CreationTime.After(allOrders[j].CreationTime.Time())
	})

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

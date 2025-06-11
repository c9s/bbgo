package dca2

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
)

type descendingClosedOrderQueryService interface {
	QueryClosedOrdersDesc(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error)
}

func recoverPosition(ctx context.Context, position *types.Position, currentRound Round, queryService types.ExchangeOrderQueryService) error {
	if position == nil {
		return fmt.Errorf("position is nil, please check it")
	}

	// reset position to recover
	position.Reset()

	var positionOrders []types.Order

	var closedTakeProfitOrderCnt int
	for _, order := range currentRound.TakeProfitOrders {
		if !types.IsActiveOrder(order) {
			closedTakeProfitOrderCnt++
		}
		positionOrders = append(positionOrders, order)
	}

	// all take-profit orders are closed, it means we will be in the new round so the position need to be reset
	if len(currentRound.TakeProfitOrders) > 0 && closedTakeProfitOrderCnt == len(currentRound.TakeProfitOrders) {
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

func recoverStartTimeOfNextRound(currentRound Round, coolDownInterval types.Duration) time.Time {
	var startTimeOfNextRound time.Time

	for _, order := range currentRound.TakeProfitOrders {
		if t := order.UpdateTime.Time().Add(coolDownInterval.Duration()); t.After(startTimeOfNextRound) {
			startTimeOfNextRound = t
		}
	}

	return startTimeOfNextRound
}

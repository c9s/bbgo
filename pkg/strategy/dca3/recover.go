package dca3

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Strategy) recover(ctx context.Context) error {
	s.logger.Info("recovering dca3")
	currentRound, err := s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	// recover profit stats
	if s.DisableProfitStatsRecover {
		s.logger.Info("disableProfitStatsRecover is set, skip profit stats recovery")
	} else {
		if err := s.recoverProfitStats(ctx); err != nil {
			return err
		}
		s.logger.Info("recover profit stats DONE")
	}

	// recover position
	if s.DisablePositionRecover {
		s.logger.Info("disablePositionRecover is set, skip position recovery")
	} else {
		if err := s.recoverPosition(ctx, currentRound, s.collector.queryService); err != nil {
			return err
		}
		s.logger.Info("recover position DONE")
	}
	s.logger.Infof("[position] current position: %s", s.Position.String())

	// recover startTimeOfNextRound
	s.recoverStartTimeOfNextRound(currentRound, s.CoolDownInterval)

	// recover state
	state, err := s.recoverState(ctx, currentRound, s.OrderExecutor)
	if err != nil {
		return err
	}

	s.stateMachine.UpdateState(state)
	s.logger.Info("recovering dca3 DONE, current state: ", s.stateMachine.GetState())

	return nil
}

func (s *Strategy) recoverPosition(ctx context.Context, currentRound Round, queryService types.ExchangeOrderQueryService) error {
	s.logger.Info("recovering position for dca3")
	if s.Position == nil {
		return fmt.Errorf("position is nil, please check it")
	}

	// reset position to recover
	s.Position.Reset()
	s.logger.Info("reset position for dca3")

	var positionOrders []types.Order

	var allOrdersClosed bool = true
	for _, order := range currentRound.OpenPositionOrders {
		if allOrdersClosed && types.IsActiveOrder(order) {
			allOrdersClosed = false
		}

		if order.ExecutedQuantity.IsZero() {
			continue
		}
		positionOrders = append(positionOrders, order)
	}
	s.logger.Info("position orders from open-position orders done")

	for _, order := range currentRound.TakeProfitOrders {
		if allOrdersClosed && types.IsActiveOrder(order) {
			allOrdersClosed = false
		}

		if order.ExecutedQuantity.IsZero() {
			continue
		}
		positionOrders = append(positionOrders, order)
	}
	s.logger.Info("position orders from take-profit orders done")

	for _, positionOrder := range positionOrders {
		s.logger.Infof("collecting trades for position order: %s", positionOrder.String())
		trades, err := retry.QueryOrderTradesUntilSuccessful(ctx, queryService, types.OrderQuery{
			Symbol:  s.Position.Symbol,
			OrderID: strconv.FormatUint(positionOrder.OrderID, 10),
		})
		s.logger.Infof("collected %d trades for position order: %s", len(trades), positionOrder.String())

		if err != nil {
			return errors.Wrapf(err, "failed to get order (%d) trades", positionOrder.OrderID)
		}
		s.Position.AddTrades(trades)
	}

	if allOrdersClosed && s.Position.GetBase().Compare(s.Market.MinQuantity) < 0 {
		s.Position.Reset()
	}

	return nil
}

func (s *Strategy) recoverProfitStats(ctx context.Context) error {
	if s.ProfitStats == nil {
		return fmt.Errorf("profit stats is nil, please check it")
	}

	_, err := s.UpdateProfitStats(ctx)
	return err
}

func (s *Strategy) recoverStartTimeOfNextRound(currentRound Round, coolDownInterval types.Duration) {
	var startTimeOfNextRound time.Time

	for _, order := range currentRound.TakeProfitOrders {
		if t := order.UpdateTime.Time().Add(coolDownInterval.Duration()); t.After(startTimeOfNextRound) {
			startTimeOfNextRound = t
		}
	}

	s.startTimeOfNextRound = startTimeOfNextRound
}

func (s *Strategy) recoverState(ctx context.Context, currentRound Round, orderExecutor *bbgo.GeneralOrderExecutor) (State, error) {
	s.logger.Info("recovering state for dca3")

	if len(currentRound.OpenPositionOrders) == 0 && len(currentRound.TakeProfitOrders) == 0 {
		return StateIdleWaiting, nil
	}

	if len(currentRound.OpenPositionOrders) == 0 && len(currentRound.TakeProfitOrders) > 0 {
		return None, fmt.Errorf("there is no open-position orders but there are take-profit orders. it should not happen, please check it")
	}

	openedOP, executedOPQuantity, _ := classfyAndCollectQuantity(currentRound.OpenPositionOrders)
	openedTP, executedTPQuantity, pendingTPQuantity := classfyAndCollectQuantity(currentRound.TakeProfitOrders)

	s.logger.Info(s.Position.String())
	s.logger.Infof("[open-position] opened: %d, executed: %f, truncated: %f", len(openedOP), executedOPQuantity.Float64(), s.Market.TruncateQuantity(executedOPQuantity).Float64())
	s.logger.Infof("[take-profit] opened: %d, executed: %f, pending: %f, truncated: %f", len(openedTP), executedTPQuantity.Float64(), pendingTPQuantity.Float64(), s.Market.TruncateQuantity(executedTPQuantity.Add(pendingTPQuantity)).Float64())

	s.addOrdersToExecutor(openedOP, openedTP, orderExecutor)

	if len(currentRound.TakeProfitOrders) == 0 {
		return s.recoverStateOpenPosition(ctx, currentRound, openedOP)
	}

	return s.recoverStateTakeProfit(ctx, currentRound, openedOP, openedTP, executedTPQuantity, pendingTPQuantity)
}

func (s *Strategy) addOrdersToExecutor(openedOP, openedTP types.OrderSlice, orderExecutor *bbgo.GeneralOrderExecutor) {
	activeOrderBook := orderExecutor.ActiveMakerOrders()
	orderStore := orderExecutor.OrderStore()
	for _, order := range openedOP {
		activeOrderBook.Add(order)
		orderStore.Add(order)
	}
	for _, order := range openedTP {
		activeOrderBook.Add(order)
		orderStore.Add(order)
	}
}

func (s *Strategy) recoverStateOpenPosition(ctx context.Context, currentRound Round, openedOP types.OrderSlice) (State, error) {
	s.logger.Info("recovering state for dca3 with only open-position orders")
	if len(currentRound.OpenPositionOrders) < int(s.MaxOrderCount) {
		return None, fmt.Errorf("there is missing open-position orders (max order count: %d, num of open-position orders: %d), please check it", s.MaxOrderCount, len(currentRound.OpenPositionOrders))
	}

	if len(openedOP) == 0 && s.Position.GetBase().Compare(s.Market.MinQuantity) < 0 {
		// no more opened open-position orders so the position can't increase any more but the position still less than min quantity.
		// it should not happen in only open-position stage
		return None, fmt.Errorf("there is no opened open-position orders but open position (%f) is less than min quantity (%f), please check it", s.Position.GetBase().Float64(), s.Market.MinQuantity.Float64())
	}

	if s.Position.GetBase().Compare(s.Market.MinQuantity) >= 0 {
		if err := s.placeTakeProfitOrder(ctx, currentRound); err != nil {
			return None, fmt.Errorf("failed to place take-profit order when recovering at open-position stage: %w", err)
		}
		return StateOpenPositionMOQReached, nil
	}
	return StateOpenPositionReady, nil
}

func (s *Strategy) recoverStateTakeProfit(
	ctx context.Context,
	currentRound Round,
	openedOP, openedTP types.OrderSlice,
	executedTPQuantity, pendingTPQuantity fixedpoint.Value,
) (State, error) {
	s.logger.Info("recovering state for dca3 with both open-position and take-profit orders")
	tpQuantity := s.Market.TruncateQuantity(s.Position.GetBase())
	if len(openedOP) == 0 && len(openedTP) == 0 {
		if tpQuantity.Compare(s.Market.MinQuantity) < 0 {
			return StateIdleWaiting, nil
		}
		if err := s.placeTakeProfitOrder(ctx, currentRound); err != nil {
			return None, fmt.Errorf("failed to place take-profit order when recovering at take-profit stage: %w", err)
		}
		return StateTakeProfitReached, nil
	}

	// if there is no executed take-profit order. it means we don't need to cancel open-position orders but only update take-profit order if needed
	if executedTPQuantity.IsZero() {
		if tpQuantity.Compare(pendingTPQuantity) > 0 {
			if err, logLevel := s.updateTakeProfitOrder(ctx); err != nil {
				switch logLevel {
				case LogLevelInfo:
					s.logger.WithError(err).Info("failed to update take-profit order when recovering at take-profit stage")
				case LogLevelWarn:
					s.logger.WithError(err).Warn("failed to update take-profit order when recovering at take-profit stage")
				case LogLevelError:
					s.logger.WithError(err).Error("failed to update take-profit order when recovering at take-profit stage")
				}

				return None, fmt.Errorf("failed to update take-profit order when recovering at take-profit stage: %w", err)
			}
		}
		return StateOpenPositionMOQReached, nil
	}

	// we need to cancel all open-position orders first because the take-profit order is reached (executedTPQuantity > 0)
	if len(openedOP) > 0 {
		if err, logLevel := s.cancelOpenPositionOrders(ctx); err != nil {
			switch logLevel {
			case LogLevelInfo:
				s.logger.WithError(err).Info("failed to cancel open-position orders when recovering at take-profit stage")
			case LogLevelWarn:
				s.logger.WithError(err).Warn("failed to cancel open-position orders when recovering at take-profit stage")
			case LogLevelError:
				s.logger.WithError(err).Error("failed to cancel open-position orders when recovering at take-profit stage")
			}
			return None, fmt.Errorf("failed to cancel open-position orders when recovering at take-profit stage: %w", err)
		}
	}

	if len(openedTP) == 0 && tpQuantity.Compare(s.Market.MinQuantity) < 0 {
		return StateIdleWaiting, nil
	}

	if tpQuantity.Compare(pendingTPQuantity) > 0 && tpQuantity.Compare(s.Market.MinQuantity) > 0 {
		if err, logLevel := s.updateTakeProfitOrder(ctx); err != nil {
			switch logLevel {
			case LogLevelInfo:
				s.logger.WithError(err).Info("failed to update take-profit order when recovering at take-profit stage")
			case LogLevelWarn:
				s.logger.WithError(err).Warn("failed to update take-profit order when recovering at take-profit stage")
			case LogLevelError:
				s.logger.WithError(err).Error("failed to update take-profit order when recovering at take-profit stage")
			}
			return None, fmt.Errorf("failed to update take-profit order when recovering at take-profit stage: %w", err)
		}
	}
	return StateTakeProfitReached, nil
}

func classfyAndCollectQuantity(orders types.OrderSlice) (opened types.OrderSlice, executedQuantity, pendingQuantity fixedpoint.Value) {
	for _, order := range orders {
		executedQuantity = executedQuantity.Add(order.ExecutedQuantity)
		switch order.Status {
		case types.OrderStatusNew, types.OrderStatusPartiallyFilled:
			opened = append(opened, order)
			pendingQuantity = pendingQuantity.Add(order.GetRemainingQuantity())
		}
	}

	return opened, executedQuantity, pendingQuantity
}

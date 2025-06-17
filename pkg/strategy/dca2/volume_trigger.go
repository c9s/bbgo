package dca2

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/dca2/statemachine"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

const (
	VolumeTriggerStateIdleWaiting statemachine.State = iota + 1
	VolumeTriggerStateOpenPositionReady
	VolumeTriggerStateOpenPositionMOQReached
	VolumeTriggerStateTakeProfitReached
)

type VolumeTriggerMode struct {
	s *Strategy

	once util.Reonce
}

func (m *VolumeTriggerMode) Bind(ctx context.Context) {
	// register state transition functions
	m.s.stateMachine.RegisterTransitionFunc(VolumeTriggerStateIdleWaiting, VolumeTriggerStateOpenPositionReady, m.openPosition)
	m.s.stateMachine.RegisterTransitionFunc(VolumeTriggerStateOpenPositionReady, VolumeTriggerStateOpenPositionMOQReached, m.placeTakeProfitOrder)
	m.s.stateMachine.RegisterTransitionFunc(VolumeTriggerStateOpenPositionMOQReached, VolumeTriggerStateOpenPositionMOQReached, m.updateTakeProfitOrder)
	m.s.stateMachine.RegisterTransitionFunc(VolumeTriggerStateOpenPositionMOQReached, VolumeTriggerStateTakeProfitReached, m.cancelOpenPositionOrders)
	m.s.stateMachine.RegisterTransitionFunc(VolumeTriggerStateOpenPositionMOQReached, VolumeTriggerStateIdleWaiting, m.finishTakeProfitStage)
	m.s.stateMachine.RegisterTransitionFunc(VolumeTriggerStateTakeProfitReached, VolumeTriggerStateIdleWaiting, m.finishTakeProfitStage)

	// state machine start callback
	m.s.stateMachine.OnStart(func() {
		var err error
		maxTry := 3
		for try := 1; try <= maxTry; try++ {
			m.s.logger.Infof("try #%d recover", try)
			err = m.recover(ctx)
			if err == nil {
				break
			}

			m.s.logger.WithError(err).Warnf("failed to recover at #%d", try)
			// sleep 10 second to retry the recovery
			time.Sleep(10 * time.Second)
		}

		// if recovery failed after maxTry attempts, the state in state machine will be None and there is no transition function will be triggered
		if err != nil {
			m.s.logger.WithError(err).Errorf("failed to recover after %d attempts, please check it", maxTry)
			return
		}

		m.s.EmitReady()
		m.triggerNextState()
	})

	// state machine close callback
	m.s.stateMachine.OnClose(func() {
		m.s.logger.Infof("state machine closed, will cancel all orders")

		var err error
		if m.s.UseCancelAllOrdersApiWhenClose {
			err = tradingutil.UniversalCancelAllOrders(ctx, m.s.ExchangeSession.Exchange, m.s.Symbol, nil)
		} else {
			err = m.s.OrderExecutor.GracefulCancel(ctx)
		}

		if err != nil {
			m.s.logger.WithError(err).Errorf("failed to cancel all orders when closing, please check it")
		}

		bbgo.Sync(ctx, m.s)
	})

	// order filled callback
	m.s.OrderExecutor.ActiveMakerOrders().OnFilled(func(o types.Order) {
		m.s.logger.Infof("FILLED ORDER: %s", o.String())

		if o.Side != TakeProfitSide {
			return
		}

		m.s.stateMachine.EmitNextState(VolumeTriggerStateIdleWaiting)
	})

	// position update callback
	m.s.OrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		m.s.logger.Infof("POSITION UPDATE: %s", position.String())

		if m.s.stateMachine.GetState() == VolumeTriggerStateOpenPositionReady && m.s.Position.GetBase().Abs().Compare(m.s.Market.MinQuantity) >= 0 {
			m.s.logger.Infof("state is VolumeTriggerStateOpenPositionReady position base (%s) is greater than or equal to min quantity (%s), emit next state", m.s.Position.GetBase().String(), m.s.Market.MinQuantity.String())
			m.s.stateMachine.EmitNextState(VolumeTriggerStateOpenPositionMOQReached)
		}
	})

	// trade event callback
	m.s.OrderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		if trade.Side == TakeProfitSide {
			m.s.stateMachine.EmitNextState(VolumeTriggerStateTakeProfitReached)
		} else if trade.Side == OpenPositionSide && m.s.stateMachine.GetState() == VolumeTriggerStateOpenPositionMOQReached {
			m.once.Do(func() {
				m.s.stateMachine.EmitNextState(VolumeTriggerStateOpenPositionMOQReached)
			})

			// TODO: need to make sure the once is reset or it will not emit next state
			time.AfterFunc(10*time.Second, func() {
				m.once.Reset()
			})
		}
	})

	// trigger next state
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				m.s.logger.Info("context done, exiting dca2 strategy")
				return
			case <-ticker.C:
				m.triggerNextState()
			}
		}
	}()
}

// openPosition will place the open-position orders
// if nextRoundPaused is set to true, it will not place the open-position orders
// if startTimeOfNextRound is not reached, it will not place the open-position orders
// if it place open-position orders successfully, it will update the state to OpenPositionReady and return true to trigger the next state immediately
func (m *VolumeTriggerMode) openPosition(ctx context.Context) error {
	m.s.logger.Info("try to open position stage")
	if m.s.nextRoundPaused {
		return fmt.Errorf("nextRoundPaused is set to true, not placing open-position orders")
	}

	if time.Now().Before(m.s.startTimeOfNextRound) {
		return fmt.Errorf("startTimeOfNextRound (%s) is not reached yet, not placing open-position orders", m.s.startTimeOfNextRound.String())
	}

	// validate the stage by orders
	currentRound, err := m.s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	if len(currentRound.OpenPositionOrders) > 0 && len(currentRound.TakeProfitOrders) == 0 {
		return fmt.Errorf("it's already in open-position stage, please check it and manually fix it")
	}

	m.s.logger.Info("place open-position orders")
	if err := m.s.placeOpenPositionOrders(ctx); err != nil {
		return fmt.Errorf("failed to place open-position orders: %w", err)
	}

	return nil
}

func (m *VolumeTriggerMode) placeTakeProfitOrder(ctx context.Context) error {
	m.s.logger.Info("try to place take-profit order stage")
	currentRound, err := m.s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	// make sure the executed quantity of open-position orders is enough
	var executedQuantity fixedpoint.Value
	for _, order := range currentRound.OpenPositionOrders {
		executedQuantity = executedQuantity.Add(order.ExecutedQuantity)
	}

	if executedQuantity.Compare(m.s.Market.MinQuantity) < 0 {
		return fmt.Errorf("executed quantity (%f) is less than min quantity (%f), not placing take-profit order", executedQuantity.Float64(), m.s.Market.MinQuantity.Float64())
	}

	if err := m.s.placeTakeProfitOrder(ctx, currentRound); err != nil {
		return fmt.Errorf("failed to place take-profit order: %w", err)
	}

	return nil
}

func (m *VolumeTriggerMode) updateTakeProfitOrder(ctx context.Context) error {
	return m.updateTakeProfitOrderWithWaitingDuration(ctx, 10*time.Second)
}

func (m *VolumeTriggerMode) updateTakeProfitOrderWithWaitingDuration(ctx context.Context, duration time.Duration) error {
	m.s.logger.Infof("try to update take-profit order stage with waiting duration %s", duration.String())
	var activeTakeProfitOrders types.OrderSlice
	orders := m.s.OrderExecutor.ActiveMakerOrders().Orders()
	for _, order := range orders {
		if types.IsActiveOrder(order) && order.Side == TakeProfitSide {
			activeTakeProfitOrders = append(activeTakeProfitOrders, order)
		}
	}

	if len(activeTakeProfitOrders) == 0 {
		m.s.logger.Warn("no active take-profit orders to update, nothing to do")
		return nil
	}

	if err := m.s.OrderExecutor.GracefulCancel(ctx, activeTakeProfitOrders...); err != nil {
		return fmt.Errorf("failed to cancel existing take-profit orders: %w", err)
	}

	// wait duration to avoid too many position updates at a time
	if duration > 0 {
		time.Sleep(duration)
	}

	return m.placeTakeProfitOrder(ctx)
}

func (m *VolumeTriggerMode) cancelOpenPositionOrders(ctx context.Context) error {
	m.s.logger.Info("try to cancel open-position orders")
	var activeOpenPositionOrders types.OrderSlice
	orders := m.s.OrderExecutor.ActiveMakerOrders().Orders()
	for _, order := range orders {
		if types.IsActiveOrder(order) && order.Side == OpenPositionSide {
			activeOpenPositionOrders = append(activeOpenPositionOrders, order)
		}
	}

	return m.s.OrderExecutor.GracefulCancel(ctx, activeOpenPositionOrders...)
}

// finishTakeProfitStage will cancel all orders, reset position, update profit stats, and emit position update
func (m *VolumeTriggerMode) finishTakeProfitStage(ctx context.Context) error {
	m.s.logger.Info("try to finish take-profit stage")
	if m.s.OrderExecutor.ActiveMakerOrders().NumOfOrders() > 0 {
		return fmt.Errorf("there are still active orders so we can't finish take-profit stage, please check it")
	}

	// cancel all orders
	if err := m.s.OrderExecutor.GracefulCancel(ctx); err != nil {
		return fmt.Errorf("failed to cancel all orders: %w", err)
	}

	// wait 3 seconds to avoid position not update
	time.Sleep(3 * time.Second)

	m.s.logger.Info("[State] finishTakeProfitStage - start resetting position and calculate quote investment for next round")

	// update profit stats
	if err := m.s.UpdateProfitStatsUntilSuccessful(ctx); err != nil {
		m.s.logger.WithError(err).Warn("failed to calculate and emit profit")
	}

	// reset position and open new round for profit stats before position opening
	m.s.Position.Reset()

	// emit position
	m.s.OrderExecutor.TradeCollector().EmitPositionUpdate(m.s.Position)

	// store into redis
	bbgo.Sync(ctx, m.s)

	// set the start time of the next round
	m.s.startTimeOfNextRound = time.Now().Add(m.s.CoolDownInterval.Duration())

	return nil
}

func (m *VolumeTriggerMode) recover(ctx context.Context) error {
	m.s.logger.Info("recovering dca2 volume trigger mode")
	currentRound, err := m.s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	// recover profit stats
	if m.s.DisableProfitStatsRecover {
		m.s.logger.Info("disableProfitStatsRecover is set, skip profit stats recovery")
	} else {
		if err := recoverProfitStats(ctx, m.s); err != nil {
			return err
		}
		m.s.logger.Info("recover profit stats DONE")
	}

	// recover position
	if m.s.DisablePositionRecover {
		m.s.logger.Info("disablePositionRecover is set, skip position recovery")
	} else {
		if err := recoverPosition(ctx, m.s.Position, currentRound, m.s.collector.queryService); err != nil {
			return err
		}
		m.s.logger.Info("recover position DONE")
	}

	// recover startTimeOfNextRound
	startTimeOfNextRound := recoverStartTimeOfNextRound(currentRound, m.s.CoolDownInterval)
	m.s.startTimeOfNextRound = startTimeOfNextRound

	// recover state
	state, err := m.recoverState(ctx, currentRound, m.s.OrderExecutor)
	if err != nil {
		return err
	}

	m.s.stateMachine.UpdateState(state)
	m.s.logger.Info("recovering dca2 volume trigger mode DONE, current state: ", m.s.stateMachine.GetState())

	m.triggerNextState()

	return nil
}

func (m *VolumeTriggerMode) recoverState(ctx context.Context, currentRound Round, orderExecutor *bbgo.GeneralOrderExecutor) (statemachine.State, error) {
	m.s.logger.Info("recovering state for dca2 volume trigger mode")
	// no open-position orders and no take-profit orders means this is the whole new strategy
	if len(currentRound.OpenPositionOrders) == 0 && len(currentRound.TakeProfitOrders) == 0 {
		return VolumeTriggerStateIdleWaiting, nil
	}

	// it should not happen
	if len(currentRound.OpenPositionOrders) == 0 && len(currentRound.TakeProfitOrders) > 0 {
		return statemachine.None, fmt.Errorf("there is no open-position orders but there are take-profit orders. it should not happen, please check it")
	}

	executedOPQuantity, _, openedOP, _, _, unexpectedOP := classfyAndCollectQuantity(currentRound.OpenPositionOrders)
	executedTPQuantity, pendingTPQuantity, openedTP, _, _, unexpectedTP := classfyAndCollectQuantity(currentRound.TakeProfitOrders)
	if len(unexpectedOP) > 0 || len(unexpectedTP) > 0 {
		return statemachine.None, fmt.Errorf("there is at least one unexpected order status in this round, please check it")
	}

	activeOrderBook := orderExecutor.ActiveMakerOrders()
	orderStore := orderExecutor.OrderStore()

	// add orders to active orderbook and order store
	for _, order := range openedOP {
		activeOrderBook.Add(order)
		orderStore.Add(order)
	}

	// add orders to active orderbook and order store
	for _, order := range openedTP {
		activeOrderBook.Add(order)
		orderStore.Add(order)
	}

	if len(currentRound.TakeProfitOrders) == 0 {
		m.s.logger.Info("recovering state for dca2 volume trigger mode with only open-position orders")
		if len(currentRound.OpenPositionOrders) < int(m.s.MaxOrderCount) {
			return statemachine.None, fmt.Errorf("there is missing open-position orders (max order count: %d, num of open-position orders: %d), please check it", m.s.MaxOrderCount, len(currentRound.OpenPositionOrders))
		}

		if len(openedOP) == 0 && executedOPQuantity.Compare(m.s.Market.MinQuantity) < 0 {
			return statemachine.None, fmt.Errorf("there is no opened open-position orders but executed quantity (%f) is less than min quantity (%f), please check it", executedOPQuantity.Float64(), m.s.Market.MinQuantity.Float64())
		}

		if executedOPQuantity.Compare(m.s.Market.MinQuantity) >= 0 {
			// if executed quantity of open-position orders is greater than or equal to min quantity, we can place take-profit order
			if err := m.s.placeTakeProfitOrder(ctx, currentRound); err != nil {
				return statemachine.None, fmt.Errorf("failed to place take-profit order when recovering at open-position stage: %w", err)
			}

			return VolumeTriggerStateOpenPositionMOQReached, nil
		} else {
			return VolumeTriggerStateOpenPositionReady, nil
		}
	}

	m.s.logger.Info("recovering state for dca2 volume trigger mode with both open-position and take-profit orders")
	if len(openedOP) == 0 && len(openedTP) == 0 {
		// if there is no opened orders for both open-position and take-profit orders, there may be two possibilities:
		// 1. all open-position orders are closed after the take-profit order is reached
		// 2. open-position orders are filled and the take-profit order is filled when the strategy is not running
		// so we need to check the executed quantity of the orders to check it
		if executedOPQuantity.Sub(executedTPQuantity).Compare(m.s.Market.MinQuantity) < 0 {
			return VolumeTriggerStateIdleWaiting, nil
		} else {
			if err := m.s.placeTakeProfitOrder(ctx, currentRound); err != nil {
				return statemachine.None, fmt.Errorf("failed to place take-profit order when recovering at take-profit stage: %w", err)
			}

			return VolumeTriggerStateTakeProfitReached, nil
		}
	}

	if executedTPQuantity.IsZero() {
		if err := m.updateTakeProfitOrderWithWaitingDuration(ctx, 0); err != nil {
			return statemachine.None, fmt.Errorf("failed to update take-profit order when recovering at take-profit stage: %w", err)
		}
		return VolumeTriggerStateOpenPositionMOQReached, nil
	} else {
		// always cancel open-position orders first because take-profit order is reached (executedTPQuantity > 0)
		if len(openedOP) > 0 {
			if err := m.cancelOpenPositionOrders(ctx); err != nil {
				return statemachine.None, fmt.Errorf("failed to cancel open-position orders when recovering at take-profit stage: %w", err)
			}
		}

		// there may be trades when the strategy is not running
		// so we need to calculate the executed quantity difference between open-position and take-profit orders to decide whether to update the take-profit order
		executedQuantityDiff := executedOPQuantity.Sub(executedTPQuantity)
		if executedQuantityDiff.Add(pendingTPQuantity).Compare(m.s.Market.MinQuantity) >= 0 {
			// the executed quantity difference add the pending quantity of take-profit orders is greater than or equal to min quantity
			// so we have enough quantity to update the take-profit order
			if err := m.updateTakeProfitOrderWithWaitingDuration(ctx, 0); err != nil {
				return statemachine.None, fmt.Errorf("failed to update take-profit order when recovering at take-profit stage: %w", err)
			}
		} else {
			// the executed quantity difference add the pending quantity of take-profit orders is less than min quantity
			// so we don't have enough quantity to update the take-profit order
			// it may have dust quantity in this strategy
			if len(openedTP) == 0 {
				return VolumeTriggerStateIdleWaiting, nil
			}
		}

		return VolumeTriggerStateTakeProfitReached, nil
	}
}

func classfyAndCollectQuantity(orders types.OrderSlice) (executedQuantity, pendingQuantity fixedpoint.Value, opened, filled, cancelled, unexpected types.OrderSlice) {
	for _, order := range orders {
		// collect executed quantity
		executedQuantity = executedQuantity.Add(order.ExecutedQuantity)

		// classify the order by its status
		switch order.Status {
		case types.OrderStatusNew, types.OrderStatusPartiallyFilled:
			opened = append(opened, order)
			pendingQuantity = pendingQuantity.Add(order.GetRemainingQuantity())
		case types.OrderStatusFilled:
			filled = append(filled, order)
		case types.OrderStatusCanceled:
			cancelled = append(cancelled, order)
		default:
			unexpected = append(unexpected, order)
		}
	}

	return executedQuantity, pendingQuantity, opened, filled, cancelled, unexpected
}

func (m *VolumeTriggerMode) triggerNextState() {
	switch m.s.stateMachine.GetState() {
	case VolumeTriggerStateIdleWaiting:
		m.s.stateMachine.EmitNextState(VolumeTriggerStateOpenPositionReady)
	case VolumeTriggerStateOpenPositionReady:
		m.s.stateMachine.EmitNextState(VolumeTriggerStateOpenPositionMOQReached)
	case VolumeTriggerStateTakeProfitReached:
		m.s.stateMachine.EmitNextState(VolumeTriggerStateIdleWaiting)
	default:
	}
}

package dca2

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/dca2/statemachine"
	"github.com/c9s/bbgo/pkg/types"
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

		if m.s.Position.GetBase().Abs().Compare(m.s.Market.MinQuantity) >= 0 {
			switch m.s.stateMachine.GetState() {
			case VolumeTriggerStateOpenPositionReady | VolumeTriggerStateOpenPositionMOQReached:
				m.s.stateMachine.EmitNextState(VolumeTriggerStateOpenPositionMOQReached)
			}
		}
	})

	// trade event callback
	m.s.OrderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		if trade.Side == TakeProfitSide {
			m.s.stateMachine.EmitNextState(VolumeTriggerStateTakeProfitReached)
		}
	})
}

// openPosition will place the open-position orders
// if nextRoundPaused is set to true, it will not place the open-position orders
// if startTimeOfNextRound is not reached, it will not place the open-position orders
// if it place open-position orders successfully, it will update the state to OpenPositionReady and return true to trigger the next state immediately
func (m *VolumeTriggerMode) openPosition(ctx context.Context) error {
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
	currentRound, err := m.s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	if err := m.s.placeTakeProfitOrder(ctx, currentRound); err != nil {
		return fmt.Errorf("failed to place take-profit order: %w", err)
	}

	return nil
}

func (m *VolumeTriggerMode) updateTakeProfitOrder(ctx context.Context) error {
	var activeTakeProfitOrders types.OrderSlice
	orders := m.s.OrderExecutor.ActiveMakerOrders().Orders()
	for _, order := range orders {
		if types.IsActiveOrder(order) && order.Side == TakeProfitSide {
			activeTakeProfitOrders = append(activeTakeProfitOrders, order)
		}
	}

	if err := m.s.OrderExecutor.GracefulCancel(ctx, activeTakeProfitOrders...); err != nil {
		return fmt.Errorf("failed to cancel existing take-profit orders: %w", err)
	}

	// wait 10 seconds to avoid too many position updates
	time.Sleep(10 * time.Second)

	return m.placeTakeProfitOrder(ctx)
}

func (m *VolumeTriggerMode) cancelOpenPositionOrders(ctx context.Context) error {
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

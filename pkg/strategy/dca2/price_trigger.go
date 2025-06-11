package dca2

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/strategy/dca2/statemachine"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

// price trigger mode
// IdleWaiting -> wait until next round starts then place open position orders
// OpenPositionReady -> wait until one open position order is filled
// OpenPositionOrderFilled -> wait until market price reaches the take profit price
// OpenPositionFinished -> cancel all open position orders and place take profit order
// TakeProfitReady -> wait until the take profit order is filled
// IdleWaiting

const (
	PriceTriggerStateIdleWaiting statemachine.State = iota + 1
	PriceTriggerStateOpenPositionReady
	PriceTriggerStateOpenPositionOrderFilled
	PriceTriggerStateOpenPositionFinished
	PriceTriggerStateTakeProfitReady
)

type PriceTriggerMode struct {
	s *Strategy
}

func NewPriceTriggerMode(strategy *Strategy) *PriceTriggerMode {
	return &PriceTriggerMode{
		s: strategy,
	}
}

func (m *PriceTriggerMode) Bind(ctx context.Context) {
	// register state transition functions
	m.s.stateMachine.RegisterTransitionFunc(PriceTriggerStateIdleWaiting, PriceTriggerStateOpenPositionReady, m.openPosition)
	m.s.stateMachine.RegisterTransitionFunc(PriceTriggerStateOpenPositionReady, PriceTriggerStateOpenPositionOrderFilled, m.readyToFinishOpenPositionStage)
	m.s.stateMachine.RegisterTransitionFunc(PriceTriggerStateOpenPositionOrderFilled, PriceTriggerStateOpenPositionFinished, m.finishOpenPositionStage)
	m.s.stateMachine.RegisterTransitionFunc(PriceTriggerStateOpenPositionFinished, PriceTriggerStateTakeProfitReady, m.cancelOpenPositionOrdersAndPlaceTakeProfitOrder)
	m.s.stateMachine.RegisterTransitionFunc(PriceTriggerStateTakeProfitReady, PriceTriggerStateIdleWaiting, m.finishTakeProfitStage)

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
		}

		m.s.updateTakeProfitPrice()

		if m.s.stateMachine.GetState() == PriceTriggerStateIdleWaiting {
			m.s.stateMachine.EmitNextState(PriceTriggerStateOpenPositionReady)
		}
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

		switch o.Side {
		case OpenPositionSide:
			m.s.stateMachine.EmitNextState(PriceTriggerStateOpenPositionOrderFilled)
		case TakeProfitSide:
			m.s.stateMachine.EmitNextState(PriceTriggerStateIdleWaiting)
		default:
			m.s.logger.Infof("unsupported side (%s) of order: %s", o.Side, o)
		}

		openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, m.s.ExchangeSession.Exchange, m.s.Symbol)
		if err != nil {
			m.s.logger.WithError(err).Warn("failed to query open orders when order filled")
			return
		}

		// update active orders metrics
		numActiveMakerOrders := m.s.OrderExecutor.ActiveMakerOrders().NumOfOrders()
		updateNumOfActiveOrdersMetrics(numActiveMakerOrders)

		if len(openOrders) != numActiveMakerOrders {
			m.s.logger.Warnf("num of open orders (%d) and active orders (%d) is different when order filled, please check it.", len(openOrders), numActiveMakerOrders)
			return
		}

		if o.Side == OpenPositionSide && numActiveMakerOrders == 0 {
			m.s.stateMachine.EmitNextState(PriceTriggerStateOpenPositionFinished)
		}
	})

	// position update callback
	m.s.OrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		m.s.logger.Infof("POSITION UPDATE: %s", m.s.Position.String())
		bbgo.Sync(ctx, m.s)

		// update take profit price here
		m.s.updateTakeProfitPrice()

		// emit position update
		m.s.EmitPositionUpdate(position)
	})

	// kline callback
	m.s.ExchangeSession.MarketDataStream.OnKLine(func(kline types.KLine) {
		if m.s.takeProfitPrice.IsZero() {
			m.s.logger.Warn("take profit price should not be 0 when there is at least one open-position order filled, please check it")
			return
		}

		if m.s.stateMachine.GetState() != PriceTriggerStateOpenPositionOrderFilled {
			return
		}

		if kline.Close.Compare(m.s.takeProfitPrice) >= 0 {
			m.s.logger.Infof("take profit price (%s) reached, will emit next state", m.s.takeProfitPrice.String())
			m.s.stateMachine.EmitNextState(PriceTriggerStateOpenPositionFinished)
		}
	})

	// trigger state transitions periodically
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				m.s.logger.Info("context done, exiting dca2 strategy")
				return
			case <-ticker.C:
				switch m.s.stateMachine.GetState() {
				case PriceTriggerStateIdleWaiting:
					m.s.stateMachine.EmitNextState(PriceTriggerStateOpenPositionReady)
				case PriceTriggerStateOpenPositionFinished:
					m.s.stateMachine.EmitNextState(PriceTriggerStateTakeProfitReady)
				}
			}
		}
	}()
}

// openPosition will place the open-position orders
// if nextRoundPaused is set to true, it will not place the open-position orders
// if startTimeOfNextRound is not reached, it will not place the open-position orders
// if it place open-position orders successfully, it will update the state to OpenPositionReady and return true to trigger the next state immediately
func (m *PriceTriggerMode) openPosition(ctx context.Context) error {
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

// readyToFinishOpenPositionStage will update the state to OpenPositionOrderFilled if there is at least one open-position order filled
// it will not trigger the next state immediately because OpenPositionOrderFilled state only trigger by kline to move to the next state
func (m *PriceTriggerMode) readyToFinishOpenPositionStage(_ context.Context) error {
	return nil
}

// finishOpenPositionStage will update the state to OpenPositionFinish and return true to trigger the next state immediately
func (m *PriceTriggerMode) finishOpenPositionStage(_ context.Context) error {
	m.s.stateMachine.EmitNextState(PriceTriggerStateTakeProfitReady)
	return nil
}

// cancelOpenPositionOrdersAndPlaceTakeProfitOrder will cancel the open-position orders and place the take-profit orders
func (m *PriceTriggerMode) cancelOpenPositionOrdersAndPlaceTakeProfitOrder(ctx context.Context) error {
	currentRound, err := m.s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	if len(currentRound.TakeProfitOrders) > 0 {
		return fmt.Errorf("there is a take-profit order, already in take-profit stage, please check it and manually fix it")
	}

	// cancel open-position orders
	if err := m.s.OrderExecutor.GracefulCancel(ctx); err != nil {
		return fmt.Errorf("failed to cancel open-position orders: %w", err)
	}

	// place the take-profit order
	if err := m.s.placeTakeProfitOrder(ctx, currentRound); err != nil {
		return fmt.Errorf("failed to place take-profit order: %w", err)
	}

	return nil
}

// finishTakeProfitStage will update the profit stats and reset the position, then wait the next round
func (m *PriceTriggerMode) finishTakeProfitStage(ctx context.Context) error {
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

func (m *PriceTriggerMode) recover(ctx context.Context) error {
	m.s.logger.Info("starting recovering")
	currentRound, err := m.s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}
	debugRoundOrders(m.s.logger, "current", currentRound)

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
	state, err := m.recoverState(currentRound, m.s.OrderExecutor)
	if err != nil {
		return err
	}

	m.s.stateMachine.UpdateState(state)
	m.s.logger.Info("recover stats DONE")

	return nil
}

func (m *PriceTriggerMode) recoverState(currentRound Round, orderExecutor *bbgo.GeneralOrderExecutor) (statemachine.State, error) {
	// no open-position orders and no take-profit orders means this is the whole new strategy
	if len(currentRound.OpenPositionOrders) == 0 && len(currentRound.TakeProfitOrders) == 0 {
		return PriceTriggerStateIdleWaiting, nil
	}

	// it should not happen
	if len(currentRound.OpenPositionOrders) == 0 && len(currentRound.TakeProfitOrders) > 0 {
		return statemachine.None, fmt.Errorf("there is no open-position orders but there are take-profit orders. it should not happen, please check it")
	}

	activeOrderBook := orderExecutor.ActiveMakerOrders()
	orderStore := orderExecutor.OrderStore()

	if len(currentRound.TakeProfitOrders) > 0 {
		return m.recoverStateIfAtTakeProfitStage(currentRound.TakeProfitOrders, activeOrderBook, orderStore)
	}

	return m.recoverStateIfAtOpenPositionStage(currentRound.OpenPositionOrders, activeOrderBook, orderStore)
}

// recoverStateIfAtTakeProfitStage will recover the state if the strategy is stopped at take-profit stage
func (m *PriceTriggerMode) recoverStateIfAtTakeProfitStage(orders types.OrderSlice, activeOrderBook *bbgo.ActiveOrderBook, orderStore *core.OrderStore) (statemachine.State, error) {
	// because we may manually recover the strategy by cancelling the orders and placing the orders again, so we don't need to consider the cancelled orders
	opened, filled, cancelled, unexpected := orders.ClassifyByStatus()

	if len(unexpected) > 0 {
		return statemachine.None, fmt.Errorf("there is unexpected status in orders %+v at take-profit stage recovery", unexpected)
	}

	// add opened order into order store
	for _, order := range opened {
		activeOrderBook.Add(order)
		orderStore.Add(order)
	}

	// no open orders and there are filled orders -> means this round is finished, will start a new round
	if len(filled) > 0 && len(opened) == 0 {
		return PriceTriggerStateIdleWaiting, nil
	}

	// there are open orders -> means this round is not finished, we still at TakeProfitReady state
	if len(opened) > 0 {
		return PriceTriggerStateTakeProfitReady, nil
	}

	// only len(opened) == 0 and len(filled) == 0 len(cancelled) > 0 will reach this line
	return statemachine.None, fmt.Errorf("the classify orders count is not expected (opened: %d, cancelled: %d, filled: %d) at take-profit stage recovery", len(opened), len(cancelled), len(filled))
}

// recoverStateIfAtOpenPositionStage will recover the state if the strategy is stopped at open-position stage
func (m *PriceTriggerMode) recoverStateIfAtOpenPositionStage(orders types.OrderSlice, activeOrderBook *bbgo.ActiveOrderBook, orderStore *core.OrderStore) (statemachine.State, error) {
	opened, filled, cancelled, unexpected := orders.ClassifyByStatus()
	if len(unexpected) > 0 {
		return statemachine.None, fmt.Errorf("there is unexpected status of orders %+v at open-position stage recovery", unexpected)
	}

	// add opened order into order store
	for _, order := range opened {
		activeOrderBook.Add(order)
		orderStore.Add(order)
	}

	// if there is any cancelled order, it means the open-position stage is finished
	if len(cancelled) > 0 {
		return PriceTriggerStateOpenPositionFinished, nil
	}

	// if there is no open order, it means all the orders were filled so this open-position stage is finished
	if len(opened) == 0 {
		return PriceTriggerStateOpenPositionFinished, nil
	}

	// if there is no filled order, it means we need to wait for at least one order filled
	if len(filled) == 0 {
		return PriceTriggerStateOpenPositionReady, nil
	}

	// if there is at least one filled order and still open orders, it means we are ready to take profit if we hit the take-profit price
	return PriceTriggerStateOpenPositionOrderFilled, nil
}

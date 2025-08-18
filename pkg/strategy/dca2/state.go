package dca2

import (
	"context"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
)

type State int64

const (
	None State = iota
	IdleWaiting
	OpenPositionReady
	OpenPositionOrderFilled
	OpenPositionFinished
	TakeProfitReady
)

var stateTransition map[State]State = map[State]State{
	IdleWaiting:             OpenPositionReady,
	OpenPositionReady:       OpenPositionOrderFilled,
	OpenPositionOrderFilled: OpenPositionFinished,
	OpenPositionFinished:    TakeProfitReady,
	TakeProfitReady:         IdleWaiting,
}

func (s *Strategy) initializeNextStateC() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	isInitialize := false
	if s.nextStateC == nil {
		s.logger.Info("[DCA] initializing next state channel")
		s.nextStateC = make(chan State, 1)
	} else {
		s.logger.Info("[DCA] nextStateC is already initialized")
		isInitialize = true
	}

	return isInitialize
}

func (s *Strategy) updateState(state State) {
	s.state = state

	s.logger.Infof("[state] update state to %d", state)

	updateStatsMetrics(state)
	updateNumOfActiveOrdersMetrics(s.OrderExecutor.ActiveMakerOrders().NumOfOrders())
}

func (s *Strategy) emitNextState(nextState State) {
	select {
	case s.nextStateC <- nextState:
	default:
		s.logger.Warn("[DCA] nextStateC is full or not initialized")
	}
}

// runState
// IdleWaiting             -> [openPosition]                                    ->
// OpenPositionReady       -> [readyToFinishOpenPositionStage] 	                ->
// OpenPositionOrderFilled -> [finishOpenPositionStage]                         ->
// OpenPositionFinish      -> [cancelOpenPositionOrdersAndPlaceTakeProfitOrder] ->
// TakeProfitReady 	       -> [finishTakeProfitStage]                           ->
func (s *Strategy) runState(ctx context.Context) {
	s.logger.Info("[DCA] runState")
	stateTriggerTicker := time.NewTicker(1 * time.Minute)
	defer stateTriggerTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("[DCA] runState DONE")
			return
		case <-stateTriggerTicker.C:
			// move triggerNextState to the end of next state handler, this ticker is used to avoid the state is stopped unexpectedly
			s.triggerNextState()
		case nextState := <-s.nextStateC:
			// next state == current state -> skip
			if nextState == s.state {
				continue
			}

			// check the next state is valid
			validNextState, exist := stateTransition[s.state]
			if !exist {
				s.logger.Warnf("[DCA] %d not in stateTransition", s.state)
				continue
			}

			if nextState != validNextState {
				s.logger.Warnf("[DCA] %d is not valid next state of curreny state %d", nextState, s.state)
				continue
			}

			// move to next state
			if triggerImmediately := s.moveToNextState(ctx, nextState); triggerImmediately {
				s.triggerNextState()
			}
		}
	}
}

func (s *Strategy) triggerNextState() {
	switch s.state {
	case OpenPositionReady:
		// only trigger from order filled event
	case OpenPositionOrderFilled:
		// only trigger from kline event
	case TakeProfitReady:
		// only trigger from order filled event
	default:
		if nextState, ok := stateTransition[s.state]; ok {
			s.emitNextState(nextState)
		}
	}
}

// moveToNextState will run the process when moving current state to next state
// it will return true if we want it trigger the next state immediately
func (s *Strategy) moveToNextState(ctx context.Context, nextState State) bool {
	switch s.state {
	case IdleWaiting:
		return s.openPosition(ctx)
	case OpenPositionReady:
		return s.readyToFinishOpenPositionStage(ctx)
	case OpenPositionOrderFilled:
		return s.finishOpenPositionStage(ctx)
	case OpenPositionFinished:
		return s.cancelOpenPositionOrdersAndPlaceTakeProfitOrder(ctx)
	case TakeProfitReady:
		return s.finishTakeProfitStage(ctx, nextState)
	}

	s.logger.Errorf("unexpected state: %d, please check it", s.state)
	return false
}

// openPosition will place the open-position orders
// if nextRoundPaused is set to true, it will not place the open-position orders
// if startTimeOfNextRound is not reached, it will not place the open-position orders
// if it place open-position orders successfully, it will update the state to OpenPositionReady and return true to trigger the next state immediately
func (s *Strategy) openPosition(ctx context.Context) bool {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	if s.nextRoundPaused {
		s.logger.Info("[State] openPosition - nextRoundPaused is set to true")
		return false
	}

	if time.Now().Before(s.startTimeOfNextRound) {
		return false
	}

	// validate the stage by orders
	s.logger.Info("[State] openPosition - validate we are not in open-position stage")
	currentRound, err := s.collector.CollectCurrentRound(ctx)
	if err != nil {
		s.logger.WithError(err).Error("openPosition - failed to collect current round")
		return false
	}

	if len(currentRound.OpenPositionOrders) > 0 && len(currentRound.TakeProfitOrders) == 0 {
		s.logger.Error("openPosition - there is an open-position order so we are already in open-position stage. please check it and manually fix it")
		return false
	}

	s.logger.Info("[State] openPosition - place open-position orders")
	if err := s.placeOpenPositionOrders(s.writeCtx); err != nil {
		if strings.Contains(err.Error(), "failed to generate open position orders") {
			s.logger.WithError(err).Warn("failed to place open-position orders, please check it.")
		} else {
			s.logger.WithError(err).Error("failed to place open-position orders, please check it.")
		}

		return false
	}

	s.updateState(OpenPositionReady)
	s.logger.Info("[State] IdleWaiting -> OpenPositionReady")
	// do not trigger next state immediately, because OpenPositionReady state only triggers by kline to move to the next state
	return false
}

// readyToFinishOpenPositionStage will update the state to OpenPositionOrderFilled if there is at least one open-position order filled
// it will not trigger the next state immediately because OpenPositionOrderFilled state only trigger by kline to move to the next state
func (s *Strategy) readyToFinishOpenPositionStage(_ context.Context) bool {
	s.updateState(OpenPositionOrderFilled)
	s.logger.Info("[State] OpenPositionReady -> OpenPositionOrderFilled")
	// do not trigger next state immediately, because OpenPositionOrderFilled state only trigger by kline to move to the next state
	return false
}

// finishOpenPositionStage will update the state to OpenPositionFinish and return true to trigger the next state immediately
func (s *Strategy) finishOpenPositionStage(_ context.Context) bool {
	s.updateState(OpenPositionFinished)
	s.logger.Info("[State] OpenPositionOrderFilled -> OpenPositionFinished")
	return true
}

// cancelOpenPositionOrdersAndPlaceTakeProfitOrder will cancel the open-position orders and place the take-profit orders
func (s *Strategy) cancelOpenPositionOrdersAndPlaceTakeProfitOrder(ctx context.Context) bool {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	// validate we are still in open-position stage
	s.logger.Info("[State] cancelOpenPositionOrdersAndPlaceTakeProfitOrder - validate we are still in open-position stage")
	currentRound, err := s.collector.CollectCurrentRound(ctx)
	if err != nil {
		s.logger.WithError(err).Error("cancelOpenPositionOrdersAndPlaceTakeProfitOrder - failed to collect current round")
		return false
	}

	if len(currentRound.TakeProfitOrders) > 0 {
		s.logger.Error("cancelOpenPositionOrdersAndPlaceTakeProfitOrder - there is a take-profit order so we are already in take-profit stage. please check it and manually fix it")
		return false
	}

	// cancel open-position orders
	s.logger.Info("[State] cancelOpenPositionOrdersAndPlaceTakeProfitOrder - cancel open-position orders")
	if err := s.OrderExecutor.GracefulCancel(ctx); err != nil {
		s.logger.WithError(err).Error("failed to cancel maker orders")
		return false
	}

	// place the take-profit order
	s.logger.Info("[State] cancelOpenPositionOrdersAndPlaceTakeProfitOrder - place the take-profit order")
	if err := s.placeTakeProfitOrder(s.writeCtx, currentRound); err != nil {
		s.logger.WithError(err).Error("failed to open take profit orders")
		return false
	}

	s.updateState(TakeProfitReady)
	s.logger.Info("[State] OpenPositionOrdersCancelled -> TakeProfitReady")
	// do not trigger next state immediately, because TakeProfitReady state only trigger by kline to move to the next state
	return false
}

// finishTakeProfitStage will update the profit stats and reset the position, then wait the next round
func (s *Strategy) finishTakeProfitStage(ctx context.Context, next State) bool {
	// wait 3 seconds to avoid position not update
	time.Sleep(3 * time.Second)

	s.logger.Info("[State] finishTakeProfitStage - start resetting position and calculate quote investment for next round")

	// update profit stats
	if err := s.UpdateProfitStatsUntilSuccessful(ctx); err != nil {
		s.logger.WithError(err).Warn("failed to calculate and emit profit")
	}

	// reset position and open new round for profit stats before position opening
	s.Position.Reset()

	// emit position
	s.OrderExecutor.TradeCollector().EmitPositionUpdate(s.Position)

	// store into redis
	bbgo.Sync(ctx, s)

	// set the start time of the next round
	s.startTimeOfNextRound = time.Now().Add(s.CoolDownInterval.Duration())
	s.updateState(IdleWaiting)
	s.logger.Infof("[State] TakeProfitReady -> IdleWaiting (startTimeOfNextRound: %s)", s.startTimeOfNextRound.String())

	return false
}

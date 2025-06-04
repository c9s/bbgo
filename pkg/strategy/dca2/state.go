package dca2

import (
	"context"
	"time"
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
	s.logger.Infof("[state] update state from %d to %d", s.state, state)
	s.state = state

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
		s.writeMutex.Lock()
		defer s.writeMutex.Unlock()

		if err := s.openPosition(ctx); err != nil {
			s.logger.WithError(err).Error("failed to open position, please check it")
			return false
		}

		s.updateState(OpenPositionReady)
		return false
	case OpenPositionReady:
		if err := s.readyToFinishOpenPositionStage(ctx); err != nil {
			s.logger.WithError(err).Error("failed to ready to finish open position stage, please check it")
			return false
		}

		s.updateState(OpenPositionOrderFilled)
		return false
	case OpenPositionOrderFilled:
		if err := s.finishOpenPositionStage(ctx); err != nil {
			s.logger.WithError(err).Error("failed to finish open position stage, please check it")
			return false
		}

		s.updateState(OpenPositionFinished)
		return true // trigger next state immediately
	case OpenPositionFinished:
		s.writeMutex.Lock()
		defer s.writeMutex.Unlock()

		if err := s.cancelOpenPositionOrdersAndPlaceTakeProfitOrder(ctx); err != nil {
			s.logger.WithError(err).Error("failed to cancel open position orders and place take profit order, please check it")
			return false
		}

		s.updateState(TakeProfitReady)
		return false
	case TakeProfitReady:
		if err := s.finishTakeProfitStage(ctx); err != nil {
			s.logger.WithError(err).Error("failed to finish take profit stage, please check it")
			return false
		}

		s.updateState(IdleWaiting)
		return false
	}

	s.logger.Errorf("unexpected state: %d, please check it", s.state)
	return false
}

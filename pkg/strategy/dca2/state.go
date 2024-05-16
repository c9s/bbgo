package dca2

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
)

const (
	openPositionRetryInterval = 10 * time.Minute
)

type State int64

const (
	None State = iota
	WaitToOpenPosition
	PositionOpening
	OpenPositionReady
	OpenPositionOrderFilled
	OpenPositionOrdersCancelling
	OpenPositionOrdersCancelled
	TakeProfitReady
)

var stateTransition map[State]State = map[State]State{
	WaitToOpenPosition:           PositionOpening,
	PositionOpening:              OpenPositionReady,
	OpenPositionReady:            OpenPositionOrderFilled,
	OpenPositionOrderFilled:      OpenPositionOrdersCancelling,
	OpenPositionOrdersCancelling: OpenPositionOrdersCancelled,
	OpenPositionOrdersCancelled:  TakeProfitReady,
	TakeProfitReady:              WaitToOpenPosition,
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
	metricsState.With(baseLabels).Set(float64(s.state))
}

func (s *Strategy) emitNextState(nextState State) {
	select {
	case s.nextStateC <- nextState:
	default:
		s.logger.Info("[DCA] nextStateC is full or not initialized")
	}
}

// runState
// WaitToOpenPosition -> after startTimeOfNextRound, place dca orders ->
// PositionOpening
// OpenPositionReady -> any dca maker order filled ->
// OpenPositionOrderFilled -> price hit the take profit ration, start cancelling ->
// OpenPositionOrdersCancelled -> place the takeProfit order ->
// TakeProfitReady -> the takeProfit order filled ->
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
	case WaitToOpenPosition:
		return s.runWaitToOpenPositionState(ctx, nextState)
	case PositionOpening:
		return s.runPositionOpening(ctx, nextState)
	case OpenPositionReady:
		return s.runOpenPositionReady(ctx, nextState)
	case OpenPositionOrderFilled:
		return s.runOpenPositionOrderFilled(ctx, nextState)
	case OpenPositionOrdersCancelling:
		return s.runOpenPositionOrdersCancelling(ctx, nextState)
	case OpenPositionOrdersCancelled:
		return s.runOpenPositionOrdersCancelled(ctx, nextState)
	case TakeProfitReady:
		return s.runTakeProfitReady(ctx, nextState)
	}

	s.logger.Errorf("unexpected state: %d, please check it", s.state)
	return false
}

func (s *Strategy) runWaitToOpenPositionState(ctx context.Context, next State) bool {
	if s.nextRoundPaused {
		s.logger.Info("[State] WaitToOpenPosition - nextRoundPaused is set")
		return false
	}

	if time.Now().Before(s.startTimeOfNextRound) {
		return false
	}

	s.updateState(PositionOpening)
	s.logger.Info("[State] WaitToOpenPosition -> PositionOpening")
	return true
}

func (s *Strategy) runPositionOpening(ctx context.Context, next State) bool {
	s.logger.Info("[State] PositionOpening - start placing open-position orders")

	if err := s.placeOpenPositionOrders(ctx); err != nil {
		s.logger.WithError(err).Error("failed to place open-position orders, please check it.")
		return false
	}

	s.updateState(OpenPositionReady)
	s.logger.Info("[State] PositionOpening -> OpenPositionReady")
	// do not trigger next state immediately, because OpenPositionReady state only trigger by kline to move to the next state
	return false
}

func (s *Strategy) runOpenPositionReady(_ context.Context, next State) bool {
	s.updateState(OpenPositionOrderFilled)
	s.logger.Info("[State] OpenPositionReady -> OpenPositionOrderFilled")
	// do not trigger next state immediately, because OpenPositionOrderFilled state only trigger by kline to move to the next state
	return false
}

func (s *Strategy) runOpenPositionOrderFilled(_ context.Context, next State) bool {
	s.updateState(OpenPositionOrdersCancelling)
	s.logger.Info("[State] OpenPositionOrderFilled -> OpenPositionOrdersCancelling")
	return true
}

func (s *Strategy) runOpenPositionOrdersCancelling(ctx context.Context, next State) bool {
	s.logger.Info("[State] OpenPositionOrdersCancelling - start cancelling open-position orders")
	if err := s.OrderExecutor.GracefulCancel(ctx); err != nil {
		s.logger.WithError(err).Error("failed to cancel maker orders")
		return false
	}
	s.updateState(OpenPositionOrdersCancelled)
	s.logger.Info("[State] OpenPositionOrdersCancelling -> OpenPositionOrdersCancelled")
	return true
}

func (s *Strategy) runOpenPositionOrdersCancelled(ctx context.Context, next State) bool {
	s.logger.Info("[State] OpenPositionOrdersCancelled - start placing take-profit orders")
	if err := s.placeTakeProfitOrders(ctx); err != nil {
		s.logger.WithError(err).Error("failed to open take profit orders")
		return false
	}
	s.updateState(TakeProfitReady)
	s.logger.Info("[State] OpenPositionOrdersCancelled -> TakeProfitReady")
	// do not trigger next state immediately, because TakeProfitReady state only trigger by kline to move to the next state
	return false
}

func (s *Strategy) runTakeProfitReady(ctx context.Context, next State) bool {
	// wait 3 seconds to avoid position not update
	time.Sleep(3 * time.Second)

	s.logger.Info("[State] TakeProfitReady - start reseting position and calculate quote investment for next round")

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
	s.updateState(WaitToOpenPosition)
	s.logger.Infof("[State] TakeProfitReady -> WaitToOpenPosition (startTimeOfNextRound: %s)", s.startTimeOfNextRound.String())

	return false
}

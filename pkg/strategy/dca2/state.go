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
	stateTriggerTicker := time.NewTicker(5 * time.Second)
	defer stateTriggerTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("[DCA] runState DONE")
			return
		case <-stateTriggerTicker.C:
			// s.logger.Infof("[DCA] triggerNextState current state: %d", s.state)
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
			switch s.state {
			case WaitToOpenPosition:
				s.runWaitToOpenPositionState(ctx, nextState)
			case PositionOpening:
				s.runPositionOpening(ctx, nextState)
			case OpenPositionReady:
				s.runOpenPositionReady(ctx, nextState)
			case OpenPositionOrderFilled:
				s.runOpenPositionOrderFilled(ctx, nextState)
			case OpenPositionOrdersCancelling:
				s.runOpenPositionOrdersCancelling(ctx, nextState)
			case OpenPositionOrdersCancelled:
				s.runOpenPositionOrdersCancelled(ctx, nextState)
			case TakeProfitReady:
				s.runTakeProfitReady(ctx, nextState)
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

func (s *Strategy) runWaitToOpenPositionState(ctx context.Context, next State) {
	if s.nextRoundPaused {
		s.logger.Info("[State] WaitToOpenPosition - nextRoundPaused is set")
		return
	}

	if time.Now().Before(s.startTimeOfNextRound) {
		return
	}

	s.updateState(PositionOpening)
	s.logger.Info("[State] WaitToOpenPosition -> PositionOpening")
}

func (s *Strategy) runPositionOpening(ctx context.Context, next State) {
	s.logger.Info("[State] PositionOpening - start placing open-position orders")

	if err := s.placeOpenPositionOrders(ctx); err != nil {
		s.logger.WithError(err).Error("failed to place open-position orders, please check it.")

		// try after 1 minute when failed to placing orders
		s.startTimeOfNextRound = s.startTimeOfNextRound.Add(openPositionRetryInterval)
		s.logger.Infof("reset startTimeOfNextRound to %s", s.startTimeOfNextRound.String())
		s.updateState(WaitToOpenPosition)
		return
	}

	s.updateState(OpenPositionReady)
	s.logger.Info("[State] PositionOpening -> OpenPositionReady")
}

func (s *Strategy) runOpenPositionReady(_ context.Context, next State) {
	s.updateState(OpenPositionOrderFilled)
	s.logger.Info("[State] OpenPositionReady -> OpenPositionOrderFilled")
}

func (s *Strategy) runOpenPositionOrderFilled(_ context.Context, next State) {
	s.updateState(OpenPositionOrdersCancelling)
	s.logger.Info("[State] OpenPositionOrderFilled -> OpenPositionOrdersCancelling")

	// after open position cancelling, immediately trigger open position cancelled to cancel the other orders
	s.emitNextState(OpenPositionOrdersCancelled)
}

func (s *Strategy) runOpenPositionOrdersCancelling(ctx context.Context, next State) {
	s.logger.Info("[State] OpenPositionOrdersCancelling - start cancelling open-position orders")
	if err := s.OrderExecutor.GracefulCancel(ctx); err != nil {
		s.logger.WithError(err).Error("failed to cancel maker orders")
		return
	}
	s.updateState(OpenPositionOrdersCancelled)
	s.logger.Info("[State] OpenPositionOrdersCancelling -> OpenPositionOrdersCancelled")

	// after open position cancelled, immediately trigger take profit ready to open take-profit order
	s.emitNextState(TakeProfitReady)
}

func (s *Strategy) runOpenPositionOrdersCancelled(ctx context.Context, next State) {
	s.logger.Info("[State] OpenPositionOrdersCancelled - start placing take-profit orders")
	if err := s.placeTakeProfitOrders(ctx); err != nil {
		s.logger.WithError(err).Error("failed to open take profit orders")
		return
	}
	s.updateState(TakeProfitReady)
	s.logger.Info("[State] OpenPositionOrdersCancelled -> TakeProfitReady")
}

func (s *Strategy) runTakeProfitReady(ctx context.Context, next State) {
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
}

package dca3

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/c9s/bbgo/pkg/util"
	"github.com/sirupsen/logrus"
)

type LogLevel int64

const (
	LogLevelNone LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelWarnFirst
	LogLevelError
)

type State int64

type TransitionHandler func(context.Context) (error, LogLevel)

const (
	None State = iota
	StateIdleWaiting
	StateOpenPositionReady
	StateOpenPositionMOQReached
	StateTakeProfitOrderReset
	StateTakeProfitReached
)

type StateMachine struct {
	once util.Reonce
	mu   sync.Mutex

	logger           *logrus.Entry
	warnFirstLoggers map[State]map[State]*util.WarnFirstLogger

	// current state
	state State
	// isRunning indicates whether the state machine is currently running.
	isRunning atomic.Bool
	// next state channel is used to emit the next state to be processed.
	nextStateC chan State
	// closeC is used to signal the state machine to stop processing.
	closeC chan struct{}
	// stateTransitionFunc is a map of state transitions, where each key is a current state
	stateTransitionFunc map[State]map[State]TransitionHandler

	// callbacks
	startCallbacks []func()
	closeCallbacks []func()
}

func NewStateMachine(logger *logrus.Entry) *StateMachine {
	s := &StateMachine{
		logger:     logger,
		nextStateC: make(chan State, 1),
		closeC:     make(chan struct{}, 1),
	}

	if s.logger == nil {
		s.logger = logrus.WithField("component", "statemachine")
	} else {
		s.logger = s.logger.WithField("component", "statemachine")
	}

	return s
}

func (s *StateMachine) OnStart(cb func()) {
	s.startCallbacks = append(s.startCallbacks, cb)
}

func (s *StateMachine) emitStart() {
	for _, cb := range s.startCallbacks {
		cb()
	}
}

func (s *StateMachine) OnClose(cb func()) {
	s.closeCallbacks = append(s.closeCallbacks, cb)
}

func (s *StateMachine) emitClose() {
	for _, cb := range s.closeCallbacks {
		cb()
	}
}

func (s *StateMachine) EmitNextState(state State) {
	s.logger.Infof("emit next state: %d", state)
	select {
	case s.nextStateC <- state:
	default:
		s.logger.Warnf("next state channel is full, skipping emit")
	}
}

func (s *StateMachine) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *StateMachine) UpdateState(state State) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Infof("update state from %d to %d", s.state, state)
	s.state = state

	updateStatsMetrics(s.state)
}

func (s *StateMachine) Close() {
	select {
	case s.closeC <- struct{}{}:
	default:
		s.logger.Warn("closeC is already closed or full")
	}
}

func (s *StateMachine) WaitForRunningIs(isRunning bool, checkInterval, timeout time.Duration) bool {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	timeoutC := time.After(timeout)

	for {
		select {
		case <-ticker.C:
			if s.isRunning.Load() == isRunning {
				return true
			}
		case <-timeoutC:
			return false
		}
	}
}

func (s *StateMachine) RegisterTransitionHandler(from State, to State, fn TransitionHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stateTransitionFunc == nil {
		s.stateTransitionFunc = make(map[State]map[State]TransitionHandler)
	}
	if s.stateTransitionFunc[from] == nil {
		s.stateTransitionFunc[from] = make(map[State]TransitionHandler)
	}
	s.stateTransitionFunc[from][to] = fn
}

func (s *StateMachine) RegisterWarnFirstLoggers(from State, to State, logger *util.WarnFirstLogger) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.warnFirstLoggers == nil {
		s.warnFirstLoggers = make(map[State]map[State]*util.WarnFirstLogger)
	}
	if s.warnFirstLoggers[from] == nil {
		s.warnFirstLoggers[from] = make(map[State]*util.WarnFirstLogger)
	}
	s.warnFirstLoggers[from][to] = logger
}

func (s *StateMachine) Run(ctx context.Context) {
	s.once.Do(func() {
		go s.runState(ctx)
	})
}

func (s *StateMachine) runState(ctx context.Context) {
	s.logger.Info("starting state machine")
	defer func() {
		s.isRunning.Store(false)
		s.once.Reset()
		s.logger.Info("state machine stopped")
	}()

	s.emitStart()
	s.isRunning.Store(true)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("context done, exiting state machine")
			return
		case <-s.closeC:
			s.logger.Info("state machine closed by request")
			s.emitClose()
			return
		case nextState := <-s.nextStateC:
			s.logger.Infof("transitioning from %d to %d", s.state, nextState)
			s.processStateTransition(ctx, nextState)
		}
	}
}

func (s *StateMachine) processStateTransition(ctx context.Context, nextState State) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if transitionMap, ok := s.stateTransitionFunc[s.state]; ok {
		if transitionFunc, ok := transitionMap[nextState]; ok && transitionFunc != nil {
			if err, logLevel := transitionFunc(ctx); err != nil {
				switch logLevel {
				case LogLevelInfo:
					s.logger.WithError(err).Infof("failed to transition from state %d to %d", s.state, nextState)
				case LogLevelWarn:
					s.logger.WithError(err).Warnf("failed to transition from state %d to %d", s.state, nextState)
				case LogLevelWarnFirst:
					if warnFirstLogger, ok := s.warnFirstLoggers[s.state][nextState]; ok {
						warnFirstLogger.WarnOrError(err, "failed to transition from state %d to %d", s.state, nextState)
					} else {
						s.logger.WithError(err).Warnf("failed to transition from state %d to %d", s.state, nextState)
					}
				case LogLevelError:
					s.logger.WithError(err).Errorf("failed to transition from state %d to %d", s.state, nextState)
				}

				return
			} else {
				s.state = nextState
			}
		} else {
			s.logger.Warnf("no transition function defined from state %d to %d", s.state, nextState)
		}
	} else {
		s.logger.Warnf("no transition functions defined for current state %d", s.state)
	}
}

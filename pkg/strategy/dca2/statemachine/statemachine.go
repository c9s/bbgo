package statemachine

import (
	"context"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/util"
	"github.com/sirupsen/logrus"
)

type State int64

type StateMachine struct {
	once   util.Reonce
	logger *logrus.Entry

	isRunning bool

	// state-related fields
	mu                  sync.Mutex
	state               State
	nextStateC          chan State
	stateTransitionFunc map[State]map[State]func(context.Context) error

	closeC chan struct{}

	recoverStateC    chan struct{}
	recoverStateFunc func(context.Context) (State, error)

	// callbacks
	startCallbacks []func()
	closeCallbacks []func()
}

func NewStateMachine(logger *logrus.Entry) *StateMachine {
	s := &StateMachine{
		logger:        logger,
		nextStateC:    make(chan State, 1),
		closeC:        make(chan struct{}, 1),
		recoverStateC: make(chan struct{}, 1),
	}

	if s.logger == nil {
		s.logger = logrus.WithField("component", "statemachine")
	} else {
		s.logger = s.logger.WithField("component", "statemachine")
	}

	return s
}

func (s *StateMachine) EmitNextState(state State) {
	select {
	case s.nextStateC <- state:
	default:
		s.logger.Warn("nextStateC is full")
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
}

// RegisterStartFunc registers a function to be called when the state machine from a to b.
func (s *StateMachine) RegisterTransitionFunc(from State, to State, fn func(context.Context) error) {
	if s.stateTransitionFunc == nil {
		s.stateTransitionFunc = make(map[State]map[State]func(context.Context) error)
	}
	if s.stateTransitionFunc[from] == nil {
		s.stateTransitionFunc[from] = make(map[State]func(context.Context) error)
	}
	s.stateTransitionFunc[from][to] = fn
}

func (s *StateMachine) SetRecoverStateFunc(fn func(context.Context) (State, error)) {
	s.recoverStateFunc = fn
}

func (s *StateMachine) Run(ctx context.Context) {
	s.once.Do(func() {
		go s.runState(ctx)
	})
}

func (s *StateMachine) Close() {
	select {
	case s.closeC <- struct{}{}:
	default:
		s.logger.Warn("closeC is already closed or full")
	}
}

func (s *StateMachine) RecoverState() {
	select {
	case s.recoverStateC <- struct{}{}:
	default:
		s.logger.Warn("recoverC is already closed or full")
	}
}

func (s *StateMachine) WaitForRunningIs(isRunning bool, checkInterval, timeout time.Duration) bool {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	timeoutC := time.After(timeout)

	for {
		select {
		case <-ticker.C:
			if s.isRunning == isRunning {
				return true
			}
		case <-timeoutC:
			return false
		}
	}
}

func (s *StateMachine) runState(ctx context.Context) {
	defer func() {
		s.isRunning = false
		s.once.Reset()
	}()

	s.isRunning = true
	s.emitStart()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("context done, exiting state machine")
			return
		case <-s.closeC:
			s.logger.Info("state machine closed")
			s.emitClose()
			return
		case <-s.recoverStateC:
			s.logger.Info("recovering state")
			if s.recoverStateFunc == nil {
				continue
			}
			recoveredState, err := s.recoverStateFunc(ctx)
			if err != nil {
				s.logger.WithError(err).Warn("failed to recover state")
				continue
			}
			s.logger.Infof("recovered state: %d", recoveredState)
			s.UpdateState(recoveredState)
		case nextState := <-s.nextStateC:
			s.logger.Infof("transitioning from %d to %d", s.state, nextState)
			if transitionMap, ok := s.stateTransitionFunc[s.state]; ok {
				if transitionFunc, ok := transitionMap[nextState]; ok && transitionFunc != nil {
					if err := transitionFunc(ctx); err != nil {
						s.logger.WithError(err).Errorf("failed to transition from state %d to %d", s.state, nextState)
						continue
					}

					s.UpdateState(nextState)
				} else {
					s.logger.Errorf("no transition function defined from state %d to %d", s.state, nextState)
				}
			} else {
				s.logger.Errorf("no transition functions defined for current state %d", s.state)
			}
		}
	}
}

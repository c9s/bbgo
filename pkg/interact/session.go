package interact

import (
	"time"

	log "github.com/sirupsen/logrus"
)

type BaseSession struct {
	OriginState  State     `json:"originState,omitempty"`
	CurrentState State     `json:"currentState,omitempty"`
	Authorized   bool      `json:"authorized,omitempty"`
	StartedTime  time.Time `json:"startedTime,omitempty"`

	// authorizing -- the user started authorizing himself/herself, do not ignore the message
	authorizing bool
}

func (s *BaseSession) SetOriginState(state State) {
	s.OriginState = state
}

func (s *BaseSession) GetOriginState() State {
	return s.OriginState
}

func (s *BaseSession) SetState(state State) {
	log.Infof("[interact] transiting state from %s -> %s", s.CurrentState, state)
	s.CurrentState = state
}

func (s *BaseSession) GetState() State {
	return s.CurrentState
}

func (s *BaseSession) SetAuthorized() {
	s.Authorized = true
	s.authorizing = false
}

func (s *BaseSession) IsAuthorized() bool {
	return s.Authorized
}

func (s *BaseSession) SetAuthorizing(b bool) {
	s.authorizing = b
}

package statemachine

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

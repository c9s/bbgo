package bbgo

import (
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type StrategyController -interface
type StrategyController struct {
	Status types.StrategyStatus

	// Callbacks
	suspendCallbacks       []func()
	resumeCallbacks        []func()
	emergencyStopCallbacks []func()
}

func (s *StrategyController) GetStatus() types.StrategyStatus {
	return s.Status
}

func (s *StrategyController) Suspend() error {
	s.Status = types.StrategyStatusStopped

	s.EmitSuspend()

	return nil
}

func (s *StrategyController) Resume() error {
	s.Status = types.StrategyStatusRunning

	s.EmitResume()

	return nil
}

func (s *StrategyController) EmergencyStop() error {
	s.Status = types.StrategyStatusStopped

	s.EmitEmergencyStop()

	return nil
}

type StrategyStatusReader interface {
	GetStatus() types.StrategyStatus
}

type StrategyToggler interface {
	StrategyStatusReader
	Suspend() error
	Resume() error
}

type EmergencyStopper interface {
	EmergencyStop() error
}

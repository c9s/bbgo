package bbgo

import (
	"github.com/c9s/bbgo/pkg/types"
)

type StrategyController struct {
	Status types.StrategyStatus

	// Callbacks
	SuspendCallback       func() error
	ResumeCallback        func() error
	EmergencyStopCallback func() error
}

func (s *StrategyController) GetStatus() types.StrategyStatus {
	return s.Status
}

func (s *StrategyController) Suspend() error {
	s.Status = types.StrategyStatusStopped

	return s.EmitSuspend()
}

func (s *StrategyController) OnSuspend(cb func() error) {
	s.SuspendCallback = cb
}

func (s *StrategyController) EmitSuspend() error {
	if s.SuspendCallback != nil {
		return s.SuspendCallback()
	} else {
		return nil
	}
}

func (s *StrategyController) Resume() error {
	s.Status = types.StrategyStatusRunning

	return s.EmitResume()
}

func (s *StrategyController) OnResume(cb func() error) {
	s.ResumeCallback = cb
}

func (s *StrategyController) EmitResume() error {
	if s.ResumeCallback != nil {
		return s.ResumeCallback()
	} else {
		return nil
	}
}

func (s *StrategyController) EmergencyStop() error {
	s.Status = types.StrategyStatusStopped

	return s.EmitEmergencyStop()
}

func (s *StrategyController) OnEmergencyStop(cb func() error) {
	s.EmergencyStopCallback = cb
}

func (s *StrategyController) EmitEmergencyStop() error {
	if s.EmergencyStopCallback != nil {
		return s.EmergencyStopCallback()
	} else {
		return nil
	}
}

type StrategyControllerInterface interface {
	GetStatus() types.StrategyStatus
	Suspend() error
	Resume() error
	EmergencyStop() error
}

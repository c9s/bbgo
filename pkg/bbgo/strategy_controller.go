package bbgo

import (
	"fmt"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"reflect"
)

type StrategyController struct {
	// Callbacks
	GetStatusCallback     func() types.StrategyStatus
	GetPositionCallback   func() *types.Position
	ClosePositionCallback func(percentage fixedpoint.Value)
	SuspendCallback       func()
	ResumeCallback        func()
	EmergencyStopCallback func()
}

func (s *StrategyController) HasCallback(callback string) bool {
	callbackV := reflect.ValueOf(s).Elem().FieldByName(callback)
	if callbackV.IsValid() {
		if !callbackV.IsNil() {
			return true
		}
	}

	return false
}

func (s *StrategyController) OnGetStatus(cb func() types.StrategyStatus) {
	s.GetStatusCallback = cb
}

func (s *StrategyController) EmitGetStatus() (status types.StrategyStatus, err error) {
	if s.GetStatusCallback != nil {
		return s.GetStatusCallback(), nil
	} else {
		return types.StrategyStatusUnknown, fmt.Errorf("no GetStatus callback registered")
	}
}

func (s *StrategyController) OnGetPosition(cb func() *types.Position) {
	s.GetPositionCallback = cb
}

func (s *StrategyController) EmitGetPosition() (position *types.Position, err error) {
	if s.GetPositionCallback != nil {
		return s.GetPositionCallback(), nil
	} else {
		return nil, fmt.Errorf("no GetPosition callback registered")
	}
}

func (s *StrategyController) OnClosePosition(cb func(percentage fixedpoint.Value)) {
	s.ClosePositionCallback = cb
}

func (s *StrategyController) EmitClosePosition(percentage fixedpoint.Value) error {
	if s.ClosePositionCallback != nil {
		s.ClosePositionCallback(percentage)
		return nil
	} else {
		return fmt.Errorf("no ClosePosition callback registered")
	}
}

func (s *StrategyController) OnSuspend(cb func()) {
	s.SuspendCallback = cb
}

func (s *StrategyController) EmitSuspend() error {
	if s.SuspendCallback != nil {
		s.SuspendCallback()
		return nil
	} else {
		return fmt.Errorf("no Suspend callback registered")
	}
}

func (s *StrategyController) OnResume(cb func()) {
	s.ResumeCallback = cb
}

func (s *StrategyController) EmitResume() error {
	if s.ResumeCallback != nil {
		s.ResumeCallback()
		return nil
	} else {
		return fmt.Errorf("no Resume callback registered")
	}
}

func (s *StrategyController) OnEmergencyStop(cb func()) {
	s.EmergencyStopCallback = cb
}

func (s *StrategyController) EmitEmergencyStop() error {
	if s.EmergencyStopCallback != nil {
		s.EmergencyStopCallback()
		return nil
	} else {
		return fmt.Errorf("no EmergencyStop callback registered")
	}
}

type StrategyControllerInterface interface {
	HasCallback(callback string) bool
	OnGetStatus(cb func() types.StrategyStatus)
	EmitGetStatus() (status types.StrategyStatus, err error)
	OnGetPosition(cb func() *types.Position)
	EmitGetPosition() (position *types.Position, err error)
	OnClosePosition(cb func(percentage fixedpoint.Value))
	EmitClosePosition(percentage fixedpoint.Value) error
	OnSuspend(cb func())
	EmitSuspend() error
	OnResume(cb func())
	EmitResume() error
	OnEmergencyStop(cb func())
	EmitEmergencyStop() error
}

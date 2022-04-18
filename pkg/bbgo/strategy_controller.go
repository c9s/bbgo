package bbgo

import (
	"fmt"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"reflect"
)

type StrategyController struct {
	Status types.StrategyStatus

	// Callbacks
	GetStatusCallback     func() types.StrategyStatus
	GetPositionCallback   func() *types.Position
	ClosePositionCallback func(percentage fixedpoint.Value) error
	SuspendCallback       func() error
	ResumeCallback        func() error
	EmergencyStopCallback func() error
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

func (s *StrategyController) OnClosePosition(cb func(percentage fixedpoint.Value) error) {
	s.ClosePositionCallback = cb
}

func (s *StrategyController) EmitClosePosition(percentage fixedpoint.Value) error {
	if s.ClosePositionCallback != nil {
		return s.ClosePositionCallback(percentage)
	} else {
		return fmt.Errorf("no ClosePosition callback registered")
	}
}

func (s *StrategyController) OnSuspend(cb func() error) {
	s.SuspendCallback = cb
}

func (s *StrategyController) EmitSuspend() error {
	if s.SuspendCallback != nil {
		return s.SuspendCallback()
	} else {
		return fmt.Errorf("no Suspend callback registered")
	}
}

func (s *StrategyController) OnResume(cb func() error) {
	s.ResumeCallback = cb
}

func (s *StrategyController) EmitResume() error {
	if s.ResumeCallback != nil {
		return s.ResumeCallback()
	} else {
		return fmt.Errorf("no Resume callback registered")
	}
}

func (s *StrategyController) OnEmergencyStop(cb func() error) {
	s.EmergencyStopCallback = cb
}

func (s *StrategyController) EmitEmergencyStop() error {
	if s.EmergencyStopCallback != nil {
		return s.EmergencyStopCallback()
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
	OnClosePosition(cb func(percentage fixedpoint.Value) error)
	EmitClosePosition(percentage fixedpoint.Value) error
	OnSuspend(cb func() error)
	EmitSuspend() error
	OnResume(cb func() error)
	EmitResume() error
	OnEmergencyStop(cb func() error)
	EmitEmergencyStop() error
}

package bbgo

import (
	"context"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type StrategyController struct {
	// Callbacks
	GetStatusCallbacks     map[string]func(ctx context.Context)
	GetPositionCallbacks   map[string]func(ctx context.Context)
	ClosePositionCallbacks map[string]func(ctx context.Context, percentage fixedpoint.Value)
	SuspendCallbacks       map[string]func(ctx context.Context)
	ResumeCallbacks        map[string]func(ctx context.Context)
	EmergencyStopCallbacks map[string]func(ctx context.Context)
}

// TODO: strategy no found

func (s *StrategyController) OnGetStatus(signature string, cb func(ctx context.Context)) {
	s.GetStatusCallbacks[signature] = cb
}

func (s *StrategyController) EmitGetStatus(signature string, ctx context.Context) {
	s.GetStatusCallbacks[signature](ctx)
}

func (s *StrategyController) OnGetPosition(signature string, cb func(ctx context.Context)) {
	s.GetPositionCallbacks[signature] = cb
}

func (s *StrategyController) OnClosePosition(signature string, cb func(ctx context.Context, percentage fixedpoint.Value)) {
	s.ClosePositionCallbacks[signature] = cb
}

func (s *StrategyController) EmitClosePosition(signature string, ctx context.Context, percentage fixedpoint.Value) {
	s.ClosePositionCallbacks[signature](ctx, percentage)
}

func (s *StrategyController) EmitGetPosition(signature string, ctx context.Context) {
	s.GetPositionCallbacks[signature](ctx)
}

func (s *StrategyController) OnSuspend(signature string, cb func(ctx context.Context)) {
	s.SuspendCallbacks[signature] = cb
}

func (s *StrategyController) EmitSuspend(signature string, ctx context.Context) {
	s.SuspendCallbacks[signature](ctx)
}

func (s *StrategyController) OnResume(signature string, cb func(ctx context.Context)) {
	s.ResumeCallbacks[signature] = cb
}

func (s *StrategyController) EmitResume(signature string, ctx context.Context) {
	s.ResumeCallbacks[signature](ctx)
}

func (s *StrategyController) OnEmergencyStop(signature string, cb func(ctx context.Context)) {
	s.EmergencyStopCallbacks[signature] = cb
}

func (s *StrategyController) EmitEmergencyStop(signature string, ctx context.Context) {
	s.EmergencyStopCallbacks[signature](ctx)
}

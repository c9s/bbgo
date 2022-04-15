package bbgo

import (
	"context"
	"github.com/c9s/bbgo/pkg/types"
)

type StrategyController struct {
	status types.StrategyStatus

	SuspendCallbacks []func(ctx context.Context)
}

func (s *StrategyController) OnSuspend(cb func(ctx context.Context)) {
	s.SuspendCallbacks = append(s.SuspendCallbacks, cb)
}

func (s *StrategyController) EmitKLineClosed(ctx context.Context) {
	for _, cb := range s.SuspendCallbacks {
		cb(ctx)
	}
}

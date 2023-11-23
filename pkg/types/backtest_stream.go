package types

import (
	"context"
)

type BacktestStream struct {
	StandardStreamEmitter
}

func (s *BacktestStream) Connect(ctx context.Context) error {
	s.EmitConnect()
	s.EmitStart()
	s.EmitAuth()
	return nil
}

func (s *BacktestStream) Close() error {
	return nil
}

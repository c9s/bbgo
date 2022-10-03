package bbgo

import (
	"context"
)

const IsolationContextKey = "bbgo"

var defaultIsolation *Isolation = nil

func init() {
	defaultIsolation = NewIsolation()
}

type Isolation struct {
	gracefulShutdown GracefulShutdown
}

func NewIsolation() *Isolation {
	return &Isolation{}
}

func NewIsolationFromContext(ctx context.Context) *Isolation {
	isolatedContext, ok := ctx.Value(IsolationContextKey).(*Isolation)
	if ok {
		return isolatedContext
	}

	return defaultIsolation
}

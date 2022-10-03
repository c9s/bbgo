package bbgo

import (
	"context"
)

const IsolationContextKey = "bbgo"

var defaultIsolation = NewIsolation()

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

func NewContextWithIsolation(parent context.Context, isolation *Isolation) context.Context {
	return context.WithValue(parent, IsolationContextKey, isolation)
}

func NewContextWithDefaultIsolation(parent context.Context) context.Context {
	return context.WithValue(parent, IsolationContextKey, defaultIsolation)
}

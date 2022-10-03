package bbgo

import (
	"context"
)

const IsolationContextKey = "bbgo"

var defaultIsolationContext *IsolationContext = nil

func init() {
	defaultIsolationContext = NewIsolation()
}

type IsolationContext struct {
	gracefulShutdown GracefulShutdown
}

func NewIsolation() *IsolationContext {
	return &IsolationContext{}
}

func NewIsolationFromContext(ctx context.Context) *IsolationContext {
	isolatedContext, ok := ctx.Value(IsolationContextKey).(*IsolationContext)
	if ok {
		return isolatedContext
	}

	return defaultIsolationContext
}

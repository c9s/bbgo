package bbgo

import (
	"context"

	"github.com/c9s/bbgo/pkg/service"
)

const IsolationContextKey = "bbgo"

var defaultIsolation = NewDefaultIsolation()

type Isolation struct {
	gracefulShutdown         GracefulShutdown
	persistenceServiceFacade *service.PersistenceServiceFacade
}

func NewDefaultIsolation() *Isolation {
	return &Isolation{
		gracefulShutdown:         GracefulShutdown{},
		persistenceServiceFacade: persistenceServiceFacade,
	}
}

func NewIsolation(persistenceFacade *service.PersistenceServiceFacade) *Isolation {
	return &Isolation{
		gracefulShutdown:         GracefulShutdown{},
		persistenceServiceFacade: persistenceFacade,
	}
}

func GetIsolationFromContext(ctx context.Context) *Isolation {
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

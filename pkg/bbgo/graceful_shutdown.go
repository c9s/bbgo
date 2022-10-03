package bbgo

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type ShutdownHandler func(ctx context.Context, wg *sync.WaitGroup)

//go:generate callbackgen -type GracefulShutdown
type GracefulShutdown struct {
	shutdownCallbacks []ShutdownHandler
}

// Shutdown is a blocking call to emit all shutdown callbacks at the same time.
func (g *GracefulShutdown) Shutdown(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(len(g.shutdownCallbacks))

	// for each shutdown callback, we give them 10 second
	shtCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

	go g.EmitShutdown(shtCtx, &wg)

	wg.Wait()
	cancel()
}

func OnShutdown(ctx context.Context, f ShutdownHandler) {
	isolatedContext := NewIsolationFromContext(ctx)
	isolatedContext.gracefulShutdown.OnShutdown(f)
}

func Shutdown(ctx context.Context) {
	logrus.Infof("shutting down...")

	isolatedContext := NewIsolationFromContext(ctx)
	todo := context.WithValue(context.TODO(), IsolationContextKey, isolatedContext)

	timeoutCtx, cancel := context.WithTimeout(todo, 30*time.Second)
	defaultIsolationContext.gracefulShutdown.Shutdown(timeoutCtx)
	cancel()
}

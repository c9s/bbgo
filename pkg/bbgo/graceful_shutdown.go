package bbgo

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
)

type ShutdownHandler func(ctx context.Context, wg *sync.WaitGroup)

//go:generate callbackgen -type GracefulShutdown
type GracefulShutdown struct {
	shutdownCallbacks []ShutdownHandler
}

// Shutdown is a blocking call to emit all shutdown callbacks at the same time.
// The context object here should not be canceled context, you need to create a todo context.
func (g *GracefulShutdown) Shutdown(shutdownCtx context.Context) {
	var wg sync.WaitGroup
	wg.Add(len(g.shutdownCallbacks))
	go g.EmitShutdown(shutdownCtx, &wg)
	wg.Wait()
}

func OnShutdown(ctx context.Context, f ShutdownHandler) {
	isolatedContext := GetIsolationFromContext(ctx)
	isolatedContext.gracefulShutdown.OnShutdown(f)
}

func Shutdown(shutdownCtx context.Context) {

	isolatedContext := GetIsolationFromContext(shutdownCtx)
	if isolatedContext == defaultIsolation {
		logrus.Infof("bbgo shutting down...")
	} else {
		logrus.Infof("bbgo shutting down (custom isolation)...")
	}

	isolatedContext.gracefulShutdown.Shutdown(shutdownCtx)
}

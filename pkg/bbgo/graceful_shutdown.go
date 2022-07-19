package bbgo

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var graceful = &Graceful{}

type ShutdownHandler func(ctx context.Context, wg *sync.WaitGroup)

//go:generate callbackgen -type Graceful
type Graceful struct {
	shutdownCallbacks []ShutdownHandler
}

// Shutdown is a blocking call to emit all shutdown callbacks at the same time.
func (g *Graceful) Shutdown(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(len(g.shutdownCallbacks))

	// for each shutdown callback, we give them 10 second
	shtCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

	go g.EmitShutdown(shtCtx, &wg)

	wg.Wait()
	cancel()
}

func OnShutdown(f ShutdownHandler) {
	graceful.OnShutdown(f)
}

func Shutdown() {
	logrus.Infof("shutting down...")

	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	graceful.Shutdown(ctx)
	cancel()
}

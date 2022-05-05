package bbgo

import (
	"context"
	"sync"
)

//go:generate callbackgen -type Graceful
type Graceful struct {
	shutdownCallbacks []func(ctx context.Context, wg *sync.WaitGroup)
}

func (g *Graceful) Shutdown(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(len(g.shutdownCallbacks))

	go g.EmitShutdown(ctx, &wg)

	wg.Wait()
}

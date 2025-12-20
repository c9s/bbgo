package sessionworker

import (
	"context"
	"sync"

	"github.com/c9s/bbgo/pkg/bbgo"
)

type Worker interface {
	Run(ctx context.Context, h *Handle)
}

type DoFunc func(ctx context.Context, h *Handle)

type Key struct {
	session *bbgo.ExchangeSession
	id      string
}

type Handle struct {
	Key

	once sync.Once

	worker Worker

	// value is used for worker function to store the result.
	value any

	mu sync.Mutex
}

func (w *Handle) Session() *bbgo.ExchangeSession {
	return w.Key.session
}

func (w *Handle) Start(ctx context.Context) {
	w.once.Do(func() {
		go w.worker.Run(ctx, w)
	})
}

func (w *Handle) SetValue(v any) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.value = v
}

func (w *Handle) Value() any {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.value
}

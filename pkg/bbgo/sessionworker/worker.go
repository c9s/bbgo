package sessionworker

import (
	"context"
	"sync"

	"github.com/c9s/bbgo/pkg/bbgo"
)

type Key struct {
	session *bbgo.ExchangeSession
	id      string
}

type DoFunc func(ctx context.Context, w *SessionWorker)

type SessionWorker struct {
	Key

	once sync.Once

	f DoFunc

	// value is used for worker function to store the result.
	value any

	mu sync.Mutex
}

func (w *SessionWorker) Session() *bbgo.ExchangeSession {
	return w.Key.session
}

func (w *SessionWorker) Start(ctx context.Context) {
	w.once.Do(func() {
		go w.f(ctx, w)
	})
}

func (w *SessionWorker) SetValue(v any) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.value = v
}

func (w *SessionWorker) Value() any {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.value
}

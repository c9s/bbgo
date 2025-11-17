package xmaker

import (
	"context"
	"sync"

	"github.com/c9s/bbgo/pkg/bbgo"
)

var sessionWorkers *sessionWorkerPool

func init() {
	sessionWorkers = &sessionWorkerPool{
		workers: make(map[sessionWorkerKey]*SessionWorker),
	}
}

type sessionWorkerKey struct {
	session *bbgo.ExchangeSession
	id      string
}

type SessionWorkerFunc func(ctx context.Context, w *SessionWorker)

type SessionWorker struct {
	sessionWorkerKey

	once sync.Once

	f SessionWorkerFunc

	// value is used for worker function to store the result.
	value any

	mu sync.Mutex
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

type sessionWorkerPool struct {
	workers map[sessionWorkerKey]*SessionWorker
	mu      sync.Mutex
}

func (p *sessionWorkerPool) Add(session *bbgo.ExchangeSession, id string, f SessionWorkerFunc) *SessionWorker {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := sessionWorkerKey{session: session, id: id}

	if _, ok := p.workers[key]; ok {
		return p.workers[key]
	}

	w := &SessionWorker{
		sessionWorkerKey: sessionWorkerKey{session: session, id: id}, f: f}

	p.workers[key] = w
	return w
}

func (p *sessionWorkerPool) Get(session *bbgo.ExchangeSession, id string) *SessionWorker {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := sessionWorkerKey{session: session, id: id}
	worker, ok := p.workers[key]
	if !ok {
		return nil
	}

	return worker
}

func (p *sessionWorkerPool) Start(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, w := range p.workers {
		w.once.Do(func() {
			go w.f(ctx, w)
		})
	}
}

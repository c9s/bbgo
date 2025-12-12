package sessionworker

import (
	"context"
	"sync"

	"github.com/c9s/bbgo/pkg/bbgo"
)

var pool = &sessionWorkerPool{
	workers: make(map[Key]*SessionWorker),
}

// Start starts a worker with the given exchange session,
// Please note the worker function will be executed immediately.
func Start(ctx context.Context, session *bbgo.ExchangeSession, id string, f DoFunc) *SessionWorker {
	worker := pool.Add(session, id, f)
	worker.Start(ctx)
	return worker
}

// Get finds and returns the worker instance from the pool
func Get(session *bbgo.ExchangeSession, id string) *SessionWorker {
	return pool.Get(session, id)
}

type sessionWorkerPool struct {
	workers map[Key]*SessionWorker
	mu      sync.Mutex
}

func (p *sessionWorkerPool) Add(session *bbgo.ExchangeSession, id string, f DoFunc) *SessionWorker {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := Key{session: session, id: id}

	if _, ok := p.workers[key]; ok {
		return p.workers[key]
	}

	w := &SessionWorker{
		Key: Key{session: session, id: id}, f: f}

	p.workers[key] = w
	return w
}

func (p *sessionWorkerPool) Get(session *bbgo.ExchangeSession, id string) *SessionWorker {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := Key{session: session, id: id}
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
		w.Start(ctx)
	}
}

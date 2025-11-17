package xmaker

import (
	"context"
	"sync"

	"github.com/c9s/bbgo/pkg/bbgo"
)

var sessionWorkers *sessionWorkerPool

func init() {
	sessionWorkers = &sessionWorkerPool{
		workers: make(map[sessionWorkerKey]*sessionWorker),
	}
}

type sessionWorkerKey struct {
	session *bbgo.ExchangeSession
	id      string
}

type sessionWorker struct {
	sessionWorkerKey

	once sync.Once

	f func(ctx context.Context)
}

type sessionWorkerPool struct {
	workers map[sessionWorkerKey]*sessionWorker
	mu      sync.Mutex
}

func (p *sessionWorkerPool) Add(session *bbgo.ExchangeSession, id string, f func(ctx context.Context)) *sessionWorker {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := sessionWorkerKey{session: session, id: id}

	if _, ok := p.workers[key]; ok {
		return p.workers[key]
	}

	w := &sessionWorker{
		sessionWorkerKey: sessionWorkerKey{session: session, id: id}, f: f}

	p.workers[key] = w
	return w
}

func (p *sessionWorkerPool) Start(ctx context.Context) {
	for _, w := range p.workers {
		w.once.Do(func() {
			go w.f(ctx)
		})
	}
}

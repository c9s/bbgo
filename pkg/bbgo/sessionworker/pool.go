package sessionworker

import (
	"context"
	"sync"

	"github.com/c9s/bbgo/pkg/bbgo"
)

var pool = &Pool{
	workers: make(map[Key]*Handle),
}

// Start starts a worker with the given exchange session,
// Please note the worker function will be executed immediately.
func Start(ctx context.Context, session *bbgo.ExchangeSession, id string, w Worker) *Handle {
	handle := pool.Add(session, id, w)
	handle.Start(ctx)
	return handle
}

// Get finds and returns the worker instance from the pool
func Get(session *bbgo.ExchangeSession, id string) *Handle {
	return pool.Get(session, id)
}

type Pool struct {
	workers map[Key]*Handle
	mu      sync.Mutex
}

func (p *Pool) Add(session *bbgo.ExchangeSession, id string, w Worker) *Handle {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := Key{session: session, id: id}

	if _, ok := p.workers[key]; ok {
		return p.workers[key]
	}

	h := &Handle{
		Key: Key{session: session, id: id},

		worker: w,
	}

	p.workers[key] = h
	return h
}

func (p *Pool) Get(session *bbgo.ExchangeSession, id string) *Handle {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := Key{session: session, id: id}
	handle, ok := p.workers[key]
	if !ok {
		return nil
	}

	return handle
}

func (p *Pool) Start(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, w := range p.workers {
		w.Start(ctx)
	}
}

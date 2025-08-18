package twap

import "sync"

type DoneSignal struct {
	doneC chan struct{}
	mu    sync.Mutex
}

func NewDoneSignal() *DoneSignal {
	return &DoneSignal{
		doneC: make(chan struct{}),
	}
}

func (e *DoneSignal) Emit() {
	e.mu.Lock()
	if e.doneC == nil {
		e.doneC = make(chan struct{})
	}

	close(e.doneC)
	e.mu.Unlock()
}

// Chan returns a channel that emits a signal when the execution is done.
func (e *DoneSignal) Chan() (c <-chan struct{}) {
	// if the channel is not allocated, it means it's not started yet, we need to return a closed channel
	e.mu.Lock()
	if e.doneC == nil {
		e.doneC = make(chan struct{})
		c = e.doneC
	} else {
		c = e.doneC
	}
	e.mu.Unlock()

	return c
}

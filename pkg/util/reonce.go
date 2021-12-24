package util

import (
	"sync"
	"sync/atomic"
)

type Reonce struct {
	done uint32
	m    sync.Mutex
}

func (o *Reonce) Reset() {
	o.m.Lock()
	atomic.StoreUint32(&o.done, 0)
	o.m.Unlock()
}

func (o *Reonce) Do(f func()) {
	if atomic.LoadUint32(&o.done) == 0 {
		// Outlined slow-path to allow inlining of the fast-path.
		o.doSlow(f)
	}
}

func (o *Reonce) doSlow(f func()) {
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}

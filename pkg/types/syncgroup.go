package types

import (
	"sync"
)

type syncGroupFunc func()

// SyncGroup is essentially a wrapper around sync.WaitGroup, designed for ease of use. You only need to use Add() to
// add routines and Run() to execute them. When it's time to close or reset, you just need to call WaitAndClear(),
// which takes care of waiting for all the routines to complete before clearing routine.
//
// It eliminates the need for manual management of sync.WaitGroup. Specifically, it highlights that SyncGroup takes
// care of sync.WaitGroup.Add() and sync.WaitGroup.Done() automatically, reducing the chances of missing these crucial calls.
type SyncGroup struct {
	wg sync.WaitGroup

	sgFuncsMu sync.Mutex
	sgFuncs   []syncGroupFunc
}

func NewSyncGroup() SyncGroup {
	return SyncGroup{}
}

func (w *SyncGroup) WaitAndClear() {
	w.wg.Wait()

	w.sgFuncsMu.Lock()
	w.sgFuncs = []syncGroupFunc{}
	w.sgFuncsMu.Unlock()
}

func (w *SyncGroup) Add(fn syncGroupFunc) {
	w.wg.Add(1)

	w.sgFuncsMu.Lock()
	w.sgFuncs = append(w.sgFuncs, fn)
	w.sgFuncsMu.Unlock()
}

func (w *SyncGroup) Run() {
	w.sgFuncsMu.Lock()
	fns := w.sgFuncs
	w.sgFuncsMu.Unlock()

	for _, fn := range fns {
		go func(doFunc syncGroupFunc) {
			defer w.wg.Done()
			doFunc()
		}(fn)
	}
}

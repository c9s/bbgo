package fixedpoint

import "sync"

type MutexValue struct {
	value Value
	mu    sync.Mutex
}

func (f *MutexValue) Add(v Value) Value {
	f.mu.Lock()
	f.value = f.value.Add(v)
	ret := f.value
	f.mu.Unlock()
	return ret
}

func (f *MutexValue) Sub(v Value) Value {
	f.mu.Lock()
	f.value = f.value.Sub(v)
	ret := f.value
	f.mu.Unlock()
	return ret
}

func (f *MutexValue) Set(v Value) {
	f.mu.Lock()
	f.value = v
	f.mu.Unlock()
}

func (f *MutexValue) Get() Value {
	f.mu.Lock()
	v := f.value
	f.mu.Unlock()
	return v
}

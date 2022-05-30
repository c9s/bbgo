//go:build go1.18
// +build go1.18

package ewoDgtrd

import "sync"

func tryLock(lock *sync.RWMutex) bool {
	return lock.TryLock()
}

func tryRLock(lock *sync.RWMutex) bool {
	return lock.TryRLock()
}

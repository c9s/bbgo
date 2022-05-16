//go:build !go1.18
// +build !go1.18

package ewoDgtrd

import "sync"

func tryLock(lock *sync.RWMutex) bool {
	lock.Lock()
	return true
}

//go:build !go1.18
// +build !go1.18

package util

import "sync"

func TryLock(lock *sync.RWMutex) bool {
	lock.Lock()
	return true
}

func TryRLock(lock *sync.RWMutex) bool {
	lock.RLock()
	return true
}

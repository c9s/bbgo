//go:build go1.18
// +build go1.18

package util

import "sync"

func TryLock(lock *sync.RWMutex) bool {
	return lock.TryLock()
}

func TryRLock(lock *sync.RWMutex) bool {
	return lock.TryRLock()
}

package util

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReonce_DoAndReset(t *testing.T) {
	var cnt = 0
	var reonce Reonce
	var wgAll, wg sync.WaitGroup
	wg.Add(1)
	wgAll.Add(2)
	go reonce.Do(func() {
		t.Log("once #1")
		time.Sleep(10 * time.Millisecond)
		cnt++
		wg.Done()
		wgAll.Done()
	})

	// make sure it's locked
	wg.Wait()
	t.Logf("reset")
	reonce.Reset()

	go reonce.Do(func() {
		t.Log("once #2")
		time.Sleep(10 * time.Millisecond)
		cnt++
		wgAll.Done()
	})

	wgAll.Wait()
	assert.Equal(t, 2, cnt)
}

package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReonce_DoAndReset(t *testing.T) {
	var cnt = 0
	var reonce Reonce
	go reonce.Do(func() {
		t.Log("once #1")
		time.Sleep(10 * time.Millisecond)
		cnt++
	})

	// make sure it's locked
	time.Sleep(10 * time.Millisecond)
	t.Logf("reset")
	reonce.Reset()

	go reonce.Do(func() {
		t.Log("once #2")
		time.Sleep(10 * time.Millisecond)
		cnt++
	})

	time.Sleep(time.Second)
	assert.Equal(t, 2, cnt)
}

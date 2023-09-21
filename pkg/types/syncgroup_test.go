package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_waitGroup_Run(t *testing.T) {
	closeCh1 := make(chan struct{})
	closeCh2 := make(chan struct{})

	wg := NewSyncGroup()
	wg.Add(func() {
		<-closeCh1
	})

	wg.Add(func() {
		<-closeCh2
	})

	wg.Run()

	close(closeCh1)
	close(closeCh2)
	wg.WaitAndClear()

	assert.Len(t, wg.sgFuncs, 0)
}

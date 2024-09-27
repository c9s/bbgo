package types

import (
	"context"
	"testing"
	"time"
)

func TestConnectivityGroup(t *testing.T) {
	ctx := context.Background()
	conn1 := NewConnectivity()
	conn2 := NewConnectivity()
	group := NewConnectivityGroup(conn1, conn2)
	allAuthedC := group.AllAuthedC(ctx)

	time.Sleep(100 * time.Millisecond)
	conn1.handleConnect()
	waitSigChan(t, conn1.ConnectedC(), time.Second)
	conn1.handleAuth()
	waitSigChan(t, conn1.AuthedC(), time.Second)

	time.Sleep(100 * time.Millisecond)
	conn2.handleConnect()
	waitSigChan(t, conn2.ConnectedC(), time.Second)

	conn2.handleAuth()
	waitSigChan(t, conn2.AuthedC(), time.Second)

	waitSigChan(t, allAuthedC, time.Second)
}

func waitSigChan(t *testing.T, c <-chan struct{}, timeoutDuration time.Duration) {
	select {
	case <-time.After(timeoutDuration):
		t.Log("timeout")
		t.Fail()

	case <-c:
		t.Log("signal received")
	}
}

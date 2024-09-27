package types

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConnectivity(t *testing.T) {
	t.Run("general", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.handleConnect()
		conn1.handleAuth()
		conn1.handleDisconnect()
	})

	t.Run("reconnect", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.handleConnect()
		conn1.handleAuth()
		conn1.handleDisconnect()

		conn1.handleConnect()
		conn1.handleAuth()
		conn1.handleDisconnect()
	})

	t.Run("no-auth reconnect", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.handleConnect()
		conn1.handleDisconnect()

		conn1.handleConnect()
		conn1.handleDisconnect()
	})
}

func TestConnectivityGroupAuthC(t *testing.T) {
	timeout := 100 * time.Millisecond
	delay := timeout * 2

	ctx := context.Background()
	conn1 := NewConnectivity()
	conn2 := NewConnectivity()
	group := NewConnectivityGroup(conn1, conn2)
	allAuthedC := group.AllAuthedC(ctx, time.Second)

	time.Sleep(delay)
	conn1.handleConnect()
	assert.True(t, waitSigChan(conn1.ConnectedC(), timeout))
	conn1.handleAuth()
	assert.True(t, waitSigChan(conn1.AuthedC(), timeout))

	time.Sleep(delay)
	conn2.handleConnect()
	assert.True(t, waitSigChan(conn2.ConnectedC(), timeout))

	conn2.handleAuth()
	assert.True(t, waitSigChan(conn2.AuthedC(), timeout))

	assert.True(t, waitSigChan(allAuthedC, timeout))
}

func TestConnectivityGroupReconnect(t *testing.T) {
	timeout := 100 * time.Millisecond
	delay := timeout * 2

	ctx := context.Background()
	conn1 := NewConnectivity()
	conn2 := NewConnectivity()
	group := NewConnectivityGroup(conn1, conn2)

	time.Sleep(delay)
	conn1.handleConnect()
	conn1.handleAuth()
	conn1authC := conn1.authedC

	time.Sleep(delay)
	conn2.handleConnect()
	conn2.handleAuth()

	assert.True(t, waitSigChan(group.AllAuthedC(ctx, time.Second), timeout), "all connections are authenticated")

	assert.False(t, group.AnyDisconnected(ctx))

	// this should re-allocate authedC
	conn1.handleDisconnect()
	assert.NotEqual(t, conn1authC, conn1.authedC)

	assert.True(t, group.AnyDisconnected(ctx))

	assert.False(t, waitSigChan(group.AllAuthedC(ctx, time.Second), timeout), "one connection should be un-authed")

	time.Sleep(delay)

	conn1.handleConnect()
	conn1.handleAuth()
	assert.True(t, waitSigChan(group.AllAuthedC(ctx, time.Second), timeout), "all connections are authenticated, again")
}

func waitSigChan(c <-chan struct{}, timeoutDuration time.Duration) bool {
	select {
	case <-time.After(timeoutDuration):
		return false

	case <-c:
		return true
	}
}

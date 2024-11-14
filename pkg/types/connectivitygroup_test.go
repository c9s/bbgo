package types

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConnectivityGroupAuthC(t *testing.T) {
	timeout := 100 * time.Millisecond
	delay := timeout * 2

	ctx := context.Background()
	conn1 := NewConnectivity()
	conn2 := NewConnectivity()
	group := NewConnectivityGroup(conn1, conn2)
	allAuthedC := group.AllAuthedC(ctx)

	time.Sleep(delay)
	conn1.setConnect()
	assert.True(t, waitSigChan(conn1.ConnectedC(), timeout))
	conn1.setAuthed()
	assert.True(t, waitSigChan(conn1.AuthedC(), timeout))

	time.Sleep(delay)
	conn2.setConnect()
	assert.True(t, waitSigChan(conn2.ConnectedC(), timeout))

	conn2.setAuthed()
	assert.True(t, waitSigChan(conn2.AuthedC(), timeout))

	assert.True(t, waitSigChan(allAuthedC, timeout))
}

func TestConnectivityGroup(t *testing.T) {
	ctx := context.Background()
	_ = ctx

	t.Run("connected", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.setConnect()

		conn2 := NewConnectivity()
		conn2.setConnect()

		group := NewConnectivityGroup(conn1, conn2)
		assert.Equal(t, ConnectivityStateConnected, group.sumState)
		assert.True(t, group.IsConnected())
		assert.False(t, group.IsAuthed())
	})

	t.Run("only one connected", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.setConnect()

		conn2 := NewConnectivity()
		conn2.setDisconnect()

		group := NewConnectivityGroup(conn1, conn2)
		assert.Equal(t, ConnectivityStateDisconnected, group.sumState)
		assert.False(t, group.IsConnected())
		assert.False(t, group.IsAuthed())
	})

	t.Run("only one authed", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.setConnect()

		conn2 := NewConnectivity()
		conn2.setAuthed()

		group := NewConnectivityGroup(conn1, conn2)
		assert.Equal(t, ConnectivityStateConnected, group.sumState)
		assert.True(t, group.IsConnected())
		assert.False(t, group.IsAuthed())
	})

	t.Run("all authed", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.setAuthed()

		conn2 := NewConnectivity()
		conn2.setAuthed()

		group := NewConnectivityGroup(conn1, conn2)
		assert.Equal(t, ConnectivityStateAuthed, group.sumState)
		assert.True(t, group.IsConnected())
		assert.True(t, group.IsAuthed())
	})

	t.Run("allAuthedC", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.setDisconnect()
		conn2 := NewConnectivity()
		conn2.setDisconnect()

		// callbackState is used to test the callback state
		callbackState := ConnectivityStateDisconnected

		group := NewConnectivityGroup(conn1, conn2)
		group.OnConnect(func() {
			callbackState = ConnectivityStateConnected
		})
		group.OnAuth(func() {
			callbackState = ConnectivityStateAuthed
		})
		group.OnDisconnect(func() {
			callbackState = ConnectivityStateDisconnected
		})

		conn1.setConnect()
		conn2.setConnect()

		assert.Equal(t, ConnectivityStateConnected, group.sumState)
		assert.True(t, group.IsConnected())
		assert.False(t, group.IsAuthed())
		assert.Equal(t, ConnectivityStateConnected, callbackState)

		go func() {
			conn1.setAuthed()
			conn2.setAuthed()
		}()

		authed1 := false
		authedC1 := group.AllAuthedC(ctx)

		authed2 := false
		authedC2 := group.AllAuthedC(ctx)

		select {
		case <-authedC1:
			authed1 = true
		case <-time.After(4 * time.Second):
		}

		select {
		case <-authedC2:
			authed2 = true
		case <-time.After(4 * time.Second):
		}

		assert.True(t, authed1)
		assert.True(t, authed2)

		assert.Equal(t, ConnectivityStateAuthed, group.GetState())
		assert.True(t, group.IsConnected())
		assert.True(t, group.IsAuthed())
	})

	t.Run("allAuthedC with 2 diff groups", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.setDisconnect()
		conn2 := NewConnectivity()
		conn2.setDisconnect()

		// callbackState is used to test the callback state
		callbackState := ConnectivityStateDisconnected

		group1 := NewConnectivityGroup(conn1, conn2)
		group1.OnConnect(func() {
			callbackState = ConnectivityStateConnected
		})
		group1.OnAuth(func() {
			callbackState = ConnectivityStateAuthed
		})
		group1.OnDisconnect(func() {
			callbackState = ConnectivityStateDisconnected
		})

		group2 := NewConnectivityGroup(conn1, conn2)

		conn1.setConnect()
		conn2.setConnect()

		assert.Equal(t, ConnectivityStateConnected, group1.GetState())
		assert.True(t, group1.IsConnected())
		assert.False(t, group1.IsAuthed())
		assert.Equal(t, ConnectivityStateConnected, callbackState)

		go func() {
			conn1.setAuthed()
			conn2.setAuthed()
		}()

		authed1 := false
		authedC1 := group1.AllAuthedC(ctx)

		authed2 := false
		authedC2 := group2.AllAuthedC(ctx)

		select {
		case <-authedC1:
			authed1 = true
		case <-time.After(4 * time.Second):
		}

		select {
		case <-authedC2:
			authed2 = true
		case <-time.After(4 * time.Second):
		}

		assert.True(t, authed1)
		assert.True(t, authed2)

		assert.Equal(t, ConnectivityStateAuthed, group1.GetState())
		assert.True(t, group1.IsConnected())
		assert.True(t, group1.IsAuthed())

		assert.Equal(t, ConnectivityStateAuthed, group2.GetState())
		assert.True(t, group2.IsConnected())
		assert.True(t, group2.IsAuthed())
	})

	t.Run("reconnect", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.setDisconnect()
		conn2 := NewConnectivity()
		conn2.setDisconnect()

		// callbackState is used to test the callback state
		callbackState := ConnectivityStateDisconnected

		group := NewConnectivityGroup(conn1, conn2)
		group.OnConnect(func() {
			callbackState = ConnectivityStateConnected
		})
		group.OnAuth(func() {
			callbackState = ConnectivityStateAuthed
		})
		group.OnDisconnect(func() {
			callbackState = ConnectivityStateDisconnected
		})

		assert.Equal(t, ConnectivityStateDisconnected, group.sumState)
		assert.False(t, group.IsConnected())
		assert.False(t, group.IsAuthed())
		assert.Equal(t, ConnectivityStateDisconnected, callbackState)

		t.Log("conn1 connected")
		conn1.setConnect()
		assert.Equal(t, ConnectivityStateDisconnected, group.sumState)
		assert.False(t, group.IsConnected())
		assert.False(t, group.IsAuthed())
		assert.Equal(t, ConnectivityStateDisconnected, callbackState)

		t.Log("conn2 connected")
		conn2.setConnect()
		assert.Equal(t, ConnectivityStateConnected, group.sumState)
		assert.True(t, group.IsConnected())
		assert.False(t, group.IsAuthed())
		assert.Equal(t, ConnectivityStateConnected, callbackState)

		t.Log("conn1 and conn2 authed")
		conn1.setAuthed()
		conn2.setAuthed()

		assert.Equal(t, ConnectivityStateAuthed, group.sumState)
		assert.True(t, group.IsConnected())
		assert.True(t, group.IsAuthed())
		assert.Equal(t, ConnectivityStateAuthed, callbackState)

		t.Log("one connection gets disconnected should fallback to disconnected state")
		conn2.setDisconnect()

		assert.Equal(t, ConnectivityStateDisconnected, group.sumState)
		assert.False(t, group.IsConnected())
		assert.False(t, group.IsAuthed())
		assert.Equal(t, ConnectivityStateDisconnected, callbackState)

		t.Log("all connections get disconnected should fallback to disconnected state")
		conn1.setDisconnect()
		conn2.setDisconnect()

		assert.Equal(t, ConnectivityStateDisconnected, group.sumState)
		assert.False(t, group.IsConnected())
		assert.False(t, group.IsAuthed())
		assert.Equal(t, ConnectivityStateDisconnected, callbackState)

		t.Log("all connections are connected again")
		conn1.setConnect()
		conn2.setConnect()

		assert.Equal(t, ConnectivityStateConnected, group.sumState)
		assert.True(t, group.IsConnected())
		assert.False(t, group.IsAuthed())
		assert.Equal(t, ConnectivityStateConnected, callbackState)

	})
}

func Test_sumStates(t *testing.T) {
	type args struct {
		states map[*Connectivity]ConnectivityState
	}
	tests := []struct {
		name string
		args args
		want ConnectivityState
	}{
		{
			name: "all connected",
			args: args{
				states: map[*Connectivity]ConnectivityState{
					NewConnectivity(): ConnectivityStateConnected,
					NewConnectivity(): ConnectivityStateConnected,
					NewConnectivity(): ConnectivityStateConnected,
				},
			},
			want: ConnectivityStateConnected,
		},
		{
			name: "only one connected should return disconnected",
			args: args{
				states: map[*Connectivity]ConnectivityState{
					NewConnectivity(): ConnectivityStateConnected,
					NewConnectivity(): ConnectivityStateDisconnected,
					NewConnectivity(): ConnectivityStateDisconnected,
				},
			},
			want: ConnectivityStateDisconnected,
		},
		{
			name: "only one connected and others authed should return connected",
			args: args{
				states: map[*Connectivity]ConnectivityState{
					NewConnectivity(): ConnectivityStateConnected,
					NewConnectivity(): ConnectivityStateAuthed,
					NewConnectivity(): ConnectivityStateAuthed,
				},
			},
			want: ConnectivityStateConnected,
		},
		{
			name: "all disconnected should return disconnected",
			args: args{
				states: map[*Connectivity]ConnectivityState{
					NewConnectivity(): ConnectivityStateDisconnected,
					NewConnectivity(): ConnectivityStateDisconnected,
					NewConnectivity(): ConnectivityStateDisconnected,
				},
			},
			want: ConnectivityStateDisconnected,
		},
		{
			name: "one disconnected should return disconnected",
			args: args{
				states: map[*Connectivity]ConnectivityState{
					NewConnectivity(): ConnectivityStateConnected,
					NewConnectivity(): ConnectivityStateConnected,
					NewConnectivity(): ConnectivityStateDisconnected,
				},
			},
			want: ConnectivityStateDisconnected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, sumStates(tt.args.states), "sumStates(%v)", tt.args.states)
		})
	}
}

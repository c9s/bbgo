package types

import (
	"context"
	"sync"
	"time"
)

//go:generate callbackgen -type ConnectivityGroup
type ConnectivityGroup struct {
	*Connectivity

	connections []*Connectivity
	mu          sync.Mutex
	authedC     chan struct{}

	states   map[*Connectivity]ConnectivityState
	sumState ConnectivityState

	connectCallbacks    []func()
	disconnectCallbacks []func()
	authCallbacks       []func()
}

type ConnectivityState int

const (
	ConnectivityStateDisconnected ConnectivityState = -1
	ConnectivityStateUnknown      ConnectivityState = 0
	ConnectivityStateConnected    ConnectivityState = 1
	ConnectivityStateAuthed       ConnectivityState = 2
)

func getConnState(con *Connectivity) ConnectivityState {
	state := ConnectivityStateDisconnected

	if con.IsAuthed() {
		state = ConnectivityStateAuthed
	} else if con.IsConnected() {
		state = ConnectivityStateConnected
	}

	return state
}

func initConnStates(cons []*Connectivity) map[*Connectivity]ConnectivityState {
	states := map[*Connectivity]ConnectivityState{}

	for _, con := range cons {
		state := ConnectivityStateDisconnected

		if con.IsAuthed() {
			state = ConnectivityStateAuthed
		} else if con.IsConnected() {
			state = ConnectivityStateConnected
		}

		states[con] = state
	}

	return states
}

func sumStates(states map[*Connectivity]ConnectivityState) ConnectivityState {
	disconnected := 0
	connected := 0
	authed := 0

	for _, state := range states {
		switch state {
		case ConnectivityStateDisconnected:
			disconnected++
		case ConnectivityStateConnected:
			connected++
		case ConnectivityStateAuthed:
			authed++
		}
	}

	numConn := len(states)

	// if one of the connections is disconnected, the group is disconnected
	if disconnected > 0 {
		return ConnectivityStateDisconnected
	} else if authed == numConn {
		// if all connections are authed, the group is authed
		return ConnectivityStateAuthed
	} else if connected == numConn || (connected+authed) == numConn {
		// if all connections are connected, the group is connected
		return ConnectivityStateConnected
	}

	return ConnectivityStateUnknown
}

func NewConnectivityGroup(cons ...*Connectivity) *ConnectivityGroup {
	states := initConnStates(cons)
	sumState := sumStates(states)
	g := &ConnectivityGroup{
		Connectivity: NewConnectivity(),
		connections:  cons,
		authedC:      make(chan struct{}),
		states:       states,
		sumState:     sumState,
	}

	for _, con := range cons {
		g.Add(con)
	}

	return g
}

func (g *ConnectivityGroup) setState(con *Connectivity, state ConnectivityState) {
	g.mu.Lock()
	g.states[con] = state
	prevState := g.sumState
	curState := sumStates(g.states)
	g.sumState = curState
	g.mu.Unlock()

	// if the state is not changed, return
	if prevState == curState {
		return
	}

	g.setFlags(curState)
	switch curState {
	case ConnectivityStateAuthed:
		g.EmitAuth()
	case ConnectivityStateConnected:
		g.EmitConnect()
	case ConnectivityStateDisconnected:
		g.EmitDisconnect()
	}
}

func (g *ConnectivityGroup) setFlags(state ConnectivityState) {
	switch state {
	case ConnectivityStateAuthed:
		g.Connectivity.setConnect()
		g.Connectivity.setAuthed()
	case ConnectivityStateConnected:
		g.Connectivity.setConnect()
	case ConnectivityStateDisconnected:
		g.Connectivity.setDisconnect()
	}
}

func (g *ConnectivityGroup) Add(con *Connectivity) {
	g.mu.Lock()
	g.connections = append(g.connections, con)
	g.states[con] = getConnState(con)
	g.sumState = sumStates(g.states)
	g.setFlags(g.sumState)
	g.mu.Unlock()

	_con := con
	con.OnDisconnect(func() {
		g.setState(_con, ConnectivityStateDisconnected)
	})

	con.OnConnect(func() {
		g.setState(_con, ConnectivityStateConnected)
	})

	con.OnAuth(func() {
		g.setState(_con, ConnectivityStateAuthed)
	})
}

func (g *ConnectivityGroup) AnyDisconnected(ctx context.Context) bool {
	g.mu.Lock()
	conns := g.connections
	g.mu.Unlock()

	for _, conn := range conns {
		select {
		case <-ctx.Done():
			return false

		case <-conn.connectedC:
			continue

		case <-conn.disconnectedC:
			return true
		}
	}

	return false
}

func (g *ConnectivityGroup) waitAllAuthed(ctx context.Context, allTimeoutDuration time.Duration) {
	g.mu.Lock()
	conns := g.connections
	c := g.authedC
	g.mu.Unlock()

	authedConns := make([]bool, len(conns))
	allTimeout := time.After(allTimeoutDuration)
	for {
		for idx, con := range conns {
			// if the connection is not authed, mark it as false
			if !con.authed {
				// authedConns[idx] = false
			}

			timeout := time.After(3 * time.Second)
			select {
			case <-ctx.Done():
				return

			case <-allTimeout:
				return

			case <-timeout:
				continue

			case <-con.AuthedC():
				authedConns[idx] = true
			}
		}

		if allTrue(authedConns) {
			close(c)
			return
		}
	}
}

// AllAuthedC returns a channel that will be closed when all connections are authenticated
// the returned channel will be closed when all connections are authenticated
// and the channel can only be used once (because we can't close a channel twice)
func (g *ConnectivityGroup) AllAuthedC(ctx context.Context, timeout time.Duration) <-chan struct{} {
	go g.waitAllAuthed(ctx, timeout)
	return g.authedC
}

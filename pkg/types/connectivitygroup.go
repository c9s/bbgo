package types

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

//go:generate callbackgen -type ConnectivityGroup
type ConnectivityGroup struct {
	*Connectivity

	connections []*Connectivity
	mu          sync.Mutex

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
		states:       states,
		sumState:     sumState,
	}

	for _, con := range cons {
		g.Add(con)
	}

	return g
}

func (g *ConnectivityGroup) GetState() (state ConnectivityState) {
	g.mu.Lock()
	state = g.sumState
	g.mu.Unlock()
	return state
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

func (g *ConnectivityGroup) DebugStates() {
	logrus.Infof("ConnectivityGroup: %d connections, state: %d",
		len(g.connections), g.GetState())

	for con, state := range g.states {
		stream := con.GetStream()
		switch state {
		case ConnectivityStateDisconnected:
			logrus.Infof("Connectivity: %T is disconnected", stream)
		case ConnectivityStateConnected:
			logrus.Infof("Connectivity: %T is connected", stream)
		case ConnectivityStateAuthed:
			logrus.Infof("Connectivity: %T is authed", stream)
		default:
			logrus.Infof("Connectivity: %T has unknown state %d", stream, state)
		}
	}
}

func (g *ConnectivityGroup) waitForState(ctx context.Context, c chan struct{}, expected ConnectivityState) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			state := g.GetState()
			if state == expected {
				close(c)
				return
			}

			time.Sleep(10 * time.Millisecond)
		}
	}
}

// AllAuthedC returns a channel that will be closed when all connections are authenticated
// the returned channel will be closed when all connections are authenticated
// and the channel can only be used once (because we can't close a channel twice)
func (g *ConnectivityGroup) AllAuthedC(ctx context.Context) <-chan struct{} {
	c := make(chan struct{})
	go g.waitForState(ctx, c, ConnectivityStateAuthed)
	return c
}

package types

import (
	"context"
	"sync"
	"time"
)

type ConnectivityGroup struct {
	connections []*Connectivity
	mu          sync.Mutex
}

func NewConnectivityGroup(cons ...*Connectivity) *ConnectivityGroup {
	return &ConnectivityGroup{
		connections: cons,
	}
}

func (g *ConnectivityGroup) Add(con *Connectivity) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.connections = append(g.connections, con)
}

func (g *ConnectivityGroup) waitAllAuthed(ctx context.Context, c chan struct{}) {
	authedConns := make([]bool, len(g.connections))
	for {
		for idx, con := range g.connections {
			timeout := time.After(3 * time.Second)

			select {
			case <-ctx.Done():
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

func (g *ConnectivityGroup) AllAuthedC(ctx context.Context) <-chan struct{} {
	c := make(chan struct{})
	go g.waitAllAuthed(ctx, c)
	return c
}

func allTrue(bools []bool) bool {
	for _, b := range bools {
		if !b {
			return false
		}
	}

	return true
}

type Connectivity struct {
	authed  bool
	authedC chan struct{}

	connected     bool
	connectedC    chan struct{}
	disconnectedC chan struct{}

	mu sync.Mutex
}

func NewConnectivity() *Connectivity {
	closedC := make(chan struct{})
	close(closedC)

	return &Connectivity{
		authed:  false,
		authedC: make(chan struct{}),

		connected:  false,
		connectedC: make(chan struct{}),

		disconnectedC: closedC,
	}
}

func (c *Connectivity) handleConnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = true
	close(c.connectedC)
	c.disconnectedC = make(chan struct{})
}

func (c *Connectivity) handleDisconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false

	if c.connectedC != nil {
		close(c.connectedC)
	}

	if c.authedC != nil {
		close(c.authedC)
	}

	c.authedC = make(chan struct{})
	c.connectedC = make(chan struct{})
	close(c.disconnectedC)
}

func (c *Connectivity) handleAuth() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.authed = true
	close(c.authedC)
}

func (c *Connectivity) AuthedC() chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authedC
}

func (c *Connectivity) ConnectedC() chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connectedC
}

func (c *Connectivity) Bind(stream Stream) {
	stream.OnConnect(c.handleConnect)
	stream.OnDisconnect(c.handleDisconnect)
	stream.OnAuth(c.handleAuth)
}

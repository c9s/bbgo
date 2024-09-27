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

func (g *ConnectivityGroup) waitAllAuthed(ctx context.Context, c chan struct{}, allTimeoutDuration time.Duration) {
	g.mu.Lock()
	conns := g.connections
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
	c := make(chan struct{})
	go g.waitAllAuthed(ctx, c, timeout)
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

func (c *Connectivity) DisconnectedC() chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.disconnectedC
}

func (c *Connectivity) Bind(stream Stream) {
	stream.OnConnect(c.handleConnect)
	stream.OnDisconnect(c.handleDisconnect)
	stream.OnAuth(c.handleAuth)
}

package types

import (
	"context"
	"fmt"
	"sync"
)

type Connector interface {
	Connect(ctx context.Context) error
}

func allTrue(bools []bool) bool {
	for _, b := range bools {
		if !b {
			return false
		}
	}

	return true
}

//go:generate callbackgen -type Connectivity
type Connectivity struct {
	authed  bool
	authedC chan struct{}

	connected     bool
	connectedC    chan struct{}
	disconnectedC chan struct{}

	connectCallbacks    []func()
	disconnectCallbacks []func()
	authCallbacks       []func()

	stream Stream
	mu     sync.Mutex
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

func (c *Connectivity) IsConnected() (conn bool) {
	c.mu.Lock()
	conn = c.connected
	c.mu.Unlock()
	return conn
}

func (c *Connectivity) GetStream() Stream {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stream
}

func (c *Connectivity) IsAuthed() (authed bool) {
	c.mu.Lock()
	authed = c.authed
	c.mu.Unlock()
	return authed
}

func (c *Connectivity) setConnect() {
	c.mu.Lock()
	if !c.connected {
		c.connected = true
		close(c.connectedC)
		c.disconnectedC = make(chan struct{})
	}
	c.mu.Unlock()
	c.EmitConnect()
}

func (c *Connectivity) setDisconnect() {
	c.mu.Lock()
	if c.connected {
		c.connected = false
		c.authed = false
		c.authedC = make(chan struct{})
		c.connectedC = make(chan struct{})
		close(c.disconnectedC)
	}
	c.mu.Unlock()
	c.EmitDisconnect()
}

func (c *Connectivity) setAuthed() {
	c.mu.Lock()
	if !c.authed {
		c.authed = true
		close(c.authedC)
	}
	c.mu.Unlock()

	c.EmitAuth()
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
	stream.OnConnect(c.setConnect)
	stream.OnDisconnect(c.setDisconnect)
	stream.OnAuth(c.setAuthed)
	c.stream = stream
}

type ConnectorManager struct {
	connectors map[Connector]Connector
}

func NewConnectorManager() *ConnectorManager {
	return &ConnectorManager{
		connectors: make(map[Connector]Connector),
	}
}

func (cm *ConnectorManager) Add(connectors ...Connector) {
	for _, connector := range connectors {
		cm.connectors[connector] = connector
	}
}

func (cm *ConnectorManager) Connect(ctx context.Context) error {
	for _, connector := range cm.connectors {
		if err := connector.Connect(ctx); err != nil {
			return fmt.Errorf("connector %T connect error: %w", connector, err)
		}
	}

	return nil
}

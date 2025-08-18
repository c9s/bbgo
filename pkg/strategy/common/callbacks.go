package common

//go:generate callbackgen -type StatusCallbacks
type StatusCallbacks struct {
	readyCallbacks  []func()
	closedCallbacks []func()
	errorCallbacks  []func(error)
}

func (c *StatusCallbacks) OnReady(cb func()) {
	c.readyCallbacks = append(c.readyCallbacks, cb)
}

func (c *StatusCallbacks) EmitReady() {
	for _, cb := range c.readyCallbacks {
		cb()
	}
}

func (c *StatusCallbacks) OnClosed(cb func()) {
	c.closedCallbacks = append(c.closedCallbacks, cb)
}

func (c *StatusCallbacks) EmitClosed() {
	for _, cb := range c.closedCallbacks {
		cb()
	}
}

func (c *StatusCallbacks) OnError(cb func(err error)) {
	c.errorCallbacks = append(c.errorCallbacks, cb)
}

func (c *StatusCallbacks) EmitError(err error) {
	for _, cb := range c.errorCallbacks {
		cb(err)
	}
}

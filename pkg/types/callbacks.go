package types

type CommonCallback struct {
	readyCallbacks  []func()
	closedCallbacks []func()
	errorCallbacks  []func(error)
}

func (c *CommonCallback) OnReady(cb func()) {
	c.readyCallbacks = append(c.readyCallbacks, cb)
}

func (c *CommonCallback) EmitReady() {
	for _, cb := range c.readyCallbacks {
		cb()
	}
}

func (c *CommonCallback) OnClosed(cb func()) {
	c.closedCallbacks = append(c.closedCallbacks, cb)
}

func (c *CommonCallback) EmitClosed() {
	for _, cb := range c.closedCallbacks {
		cb()
	}
}

func (c *CommonCallback) OnError(cb func(err error)) {
	c.errorCallbacks = append(c.errorCallbacks, cb)
}

func (c *CommonCallback) EmitError(err error) {
	for _, cb := range c.errorCallbacks {
		cb(err)
	}
}

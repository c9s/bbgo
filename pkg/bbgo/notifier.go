package bbgo

type Notifier interface {
	NotifyTo(channel, format string, args ...interface{})
	Notify(format string, args ...interface{})
}

type NullNotifier struct{}

func (n *NullNotifier) NotifyTo(channel, format string, args ...interface{}) {}

func (n *NullNotifier) Notify(format string, args ...interface{}) {}

type Notifiability struct {
	notifiers            []Notifier
	SessionChannelRouter *PatternChannelRouter
	SymbolChannelRouter  *PatternChannelRouter
	ObjectChannelRouter  *ObjectChannelRouter
}

// RouteSession routes symbol name to channel
func (m *Notifiability) RouteSymbol(symbol string) (channel string, ok bool) {
	if m.SymbolChannelRouter != nil {
		return m.SymbolChannelRouter.Route(symbol)
	}
	return "", false
}

// RouteSession routes Session name to channel
func (m *Notifiability) RouteSession(session string) (channel string, ok bool) {
	if m.SessionChannelRouter != nil {
		return m.SessionChannelRouter.Route(session)
	}
	return "", false
}

// RouteObject routes object to channel
func (m *Notifiability) RouteObject(obj interface{}) (channel string, ok bool) {
	if m.ObjectChannelRouter != nil {
		return m.ObjectChannelRouter.Route(obj)
	}
	return "", false
}

// AddNotifier adds the notifier that implements the Notifier interface.
func (m *Notifiability) AddNotifier(notifier Notifier) {
	m.notifiers = append(m.notifiers, notifier)
}

func (m *Notifiability) Notify(format string, args ...interface{}) {
	for _, n := range m.notifiers {
		n.Notify(format, args...)
	}
}

func (m *Notifiability) NotifyTo(channel, format string, args ...interface{}) {
	for _, n := range m.notifiers {
		n.NotifyTo(channel, format, args...)
	}
}

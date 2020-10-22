package bbgo

type Notifier interface {
	NotifyTo(channel, format string, args ...interface{}) error
	Notify(format string, args ...interface{}) error
}

type NullNotifier struct{}

func (n *NullNotifier) NotifyTo(channel, format string, args ...interface{}) error {
	return nil
}

func (n *NullNotifier) Notify(format string, args ...interface{}) error {
	return nil
}

type Notifiability struct {
	notifiers []Notifier
}

func (m *Notifiability) AddNotifier(notifier Notifier) {
	m.notifiers = append(m.notifiers, notifier)
}

func (m *Notifiability) Notify(msg string, args ...interface{}) {
	for _, n := range m.notifiers {
		n.NotifyTo("", msg, args...)
	}
}

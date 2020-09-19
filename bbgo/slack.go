package bbgo

type Notifier interface {
	Notify(format string, args ...interface{})
}

type NullNotifier struct{}

func (n *NullNotifier) Notify(format string, args ...interface{}) {
}


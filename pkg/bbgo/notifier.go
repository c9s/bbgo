package bbgo

type Notifier interface {
	Notify(channel, format string, args ...interface{}) error
}

type NullNotifier struct{}

func (n *NullNotifier) Notify(format string, args ...interface{}) error {
	return nil
}

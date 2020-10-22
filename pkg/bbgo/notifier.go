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

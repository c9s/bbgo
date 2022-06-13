package bbgo

import (
	"reflect"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Notifier interface {
	NotifyTo(channel string, obj interface{}, args ...interface{})
	Notify(obj interface{}, args ...interface{})
}

type NullNotifier struct{}

func (n *NullNotifier) NotifyTo(channel string, obj interface{}, args ...interface{}) {}

func (n *NullNotifier) Notify(obj interface{}, args ...interface{}) {}

type Notifiability struct {
	notifiers            []Notifier
	SessionChannelRouter *PatternChannelRouter `json:"-"`
	SymbolChannelRouter  *PatternChannelRouter `json:"-"`
	ObjectChannelRouter  *ObjectChannelRouter  `json:"-"`
}

// RouteSymbol routes symbol name to channel
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

func (m *Notifiability) Notify(obj interface{}, args ...interface{}) {
	if str, ok := obj.(string); ok {
		simpleArgs := filterSimpleArgs(args)
		logrus.Infof(str, simpleArgs...)
	}

	for _, n := range m.notifiers {
		n.Notify(obj, args...)
	}
}

func (m *Notifiability) NotifyTo(channel string, obj interface{}, args ...interface{}) {
	for _, n := range m.notifiers {
		n.NotifyTo(channel, obj, args...)
	}
}

func filterSimpleArgs(args []interface{}) (simpleArgs []interface{}) {
	for _, arg := range args {
		switch arg.(type) {
		case int, int64, int32, uint64, uint32, string, []byte, float64, float32, fixedpoint.Value:
			simpleArgs = append(simpleArgs, arg)
		default:
			rt := reflect.TypeOf(arg)
			if rt.Kind() == reflect.Ptr {
				rt = rt.Elem()
			}
		
			switch rt.Kind() {
			case reflect.Float64, reflect.Float32, reflect.String, reflect.Int, reflect.Int64, reflect.Uint64:
				simpleArgs = append(simpleArgs, arg)
			}
		}
	}

	return simpleArgs
}

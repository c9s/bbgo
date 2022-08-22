package dynamic

import (
	"reflect"
)

type InstanceIDProvider interface {
	InstanceID() string
}

func CallID(obj interface{}) string {
	sv := reflect.ValueOf(obj)
	st := reflect.TypeOf(obj)
	if st.Implements(reflect.TypeOf((*InstanceIDProvider)(nil)).Elem()) {
		m := sv.MethodByName("InstanceID")
		ret := m.Call(nil)
		return ret[0].String()
	}

	if symbol, ok := LookupSymbolField(sv); ok {
		m := sv.MethodByName("ID")
		ret := m.Call(nil)
		return ret[0].String() + ":" + symbol
	}

	// fallback to just ID
	m := sv.MethodByName("ID")
	ret := m.Call(nil)
	return ret[0].String() + ":"
}

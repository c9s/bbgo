package bbgo

import (
	"reflect"
)

type InstanceIDProvider interface {
	InstanceID() string
}

func callID(obj interface{}) string {
	sv := reflect.ValueOf(obj)
	st := reflect.TypeOf(obj)
	if st.Implements(reflect.TypeOf((*InstanceIDProvider)(nil)).Elem()) {
		m := sv.MethodByName("InstanceID")
		ret := m.Call(nil)
		return ret[0].String()
	}

	if symbol, ok := isSymbolBasedStrategy(sv); ok {
		m := sv.MethodByName("ID")
		ret := m.Call(nil)
		return ret[0].String() + ":" + symbol
	}

	// fallback to just ID
	m := sv.MethodByName("ID")
	ret := m.Call(nil)
	return ret[0].String() + ":"
}

func isSymbolBasedStrategy(rs reflect.Value) (string, bool) {
	if rs.Kind() == reflect.Ptr {
		rs = rs.Elem()
	}

	field := rs.FieldByName("Symbol")
	if !field.IsValid() {
		return "", false
	}

	if field.Kind() != reflect.String {
		return "", false
	}

	return field.String(), true
}


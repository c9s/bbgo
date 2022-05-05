package bbgo

import (
	"reflect"
)

type InstanceIDProvider interface{
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
	return ""
}

func isSymbolBasedStrategy(rs reflect.Value) (string, bool) {
	field := rs.FieldByName("Symbol")
	if !field.IsValid() {
		return "", false
	}

	if field.Kind() != reflect.String {
		return "", false
	}

	return field.String(), true
}

func hasField(rs reflect.Value, fieldName string) (field reflect.Value, ok bool) {
	field = rs.FieldByName(fieldName)
	return field, field.IsValid()
}

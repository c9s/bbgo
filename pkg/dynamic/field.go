package dynamic

import "reflect"

func HasField(rs reflect.Value, fieldName string) (field reflect.Value, ok bool) {
	field = rs.FieldByName(fieldName)
	return field, field.IsValid()
}

func LookupSymbolField(rs reflect.Value) (string, bool) {
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


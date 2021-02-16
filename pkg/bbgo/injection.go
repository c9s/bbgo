package bbgo

import (
	"fmt"
	"reflect"
)

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

func injectField(rs reflect.Value, fieldName string, obj interface{}, pointerOnly bool) error {
	field := rs.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}

	if !field.CanSet() {
		return fmt.Errorf("field %s of %s can not be set", fieldName, rs.Type())
	}

	rv := reflect.ValueOf(obj)
	if field.Kind() == reflect.Ptr {
		if field.Type() != rv.Type() {
			return fmt.Errorf("field type mismatches: %s != %s", field.Type(), rv.Type())
		}

		field.Set(rv)
	} else if field.Kind() == reflect.Interface {
		field.Set(rv)
	} else {
		// set as value
		if pointerOnly {
			return fmt.Errorf("field %s %s does not allow value assignment (pointer type only)", field.Type(), rv.Type())
		}

		field.Set(rv.Elem())
	}

	return nil
}

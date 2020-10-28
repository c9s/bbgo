package bbgo

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

func hasField(rs reflect.Value, fieldName string) bool {
	field := rs.FieldByName(fieldName)
	return field.IsValid()
}

func injectField(rs reflect.Value, fieldName string, obj interface{}) error {
	field := rs.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}

	logrus.Infof("found %s in %s, injecting %T...", fieldName, rs.Type(), obj)

	if !field.CanSet() {
		return errors.Errorf("field %s of %s can not be set", fieldName, rs.Type())
	}

	rv := reflect.ValueOf(obj)
	if field.Kind() == reflect.Ptr {
		if field.Type() != rv.Type() {
			return errors.Errorf("field type mismatches: %s != %s", field.Type(), rv.Type())
		}

		field.Set(rv)
	} else if field.Kind() == reflect.Interface {
		field.Set(rv)
	} else {
		// set as value
		field.Set(rv.Elem())
	}

	return nil
}

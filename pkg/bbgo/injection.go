package bbgo

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func injectStrategyField(strategy SingleExchangeStrategy, rs reflect.Value, fieldName string, obj interface{}) error {
	field := rs.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}

	logrus.Infof("found %s in strategy %T, injecting %T...", fieldName, strategy, obj)

	if !field.CanSet() {
		return errors.Errorf("field %s of strategy %T can not be set", fieldName, strategy)
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

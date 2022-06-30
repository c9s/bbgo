package dynamic

import (
	"errors"
	"reflect"
)

// CallStructFieldsMethod iterates field from the given struct object
// check if the field object implements the interface, if it's implemented, then we call a specific method
func CallStructFieldsMethod(m interface{}, method string, args ...interface{}) error {
	rv := reflect.ValueOf(m)
	rt := reflect.TypeOf(m)

	if rt.Kind() != reflect.Ptr {
		return errors.New("the given object needs to be a pointer")
	}

	rv = rv.Elem()
	rt = rt.Elem()

	if rt.Kind() != reflect.Struct {
		return errors.New("the given object needs to be struct")
	}

	argValues := ToReflectValues(args...)
	for i := 0; i < rt.NumField(); i++ {
		fieldType := rt.Field(i)

		// skip non-exported fields
		if !fieldType.IsExported() {
			continue
		}

		if _, ok := fieldType.Type.MethodByName(method); ok {
			refMethod := rv.Field(i).MethodByName(method)
			refMethod.Call(argValues)
		}
	}

	return nil
}

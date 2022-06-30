package dynamic

import "reflect"

// https://github.com/xiaojun207/go-base-utils/blob/master/utils/Clone.go
func NewTypeValueInterface(typ reflect.Type) interface{} {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		dst := reflect.New(typ).Elem()
		return dst.Addr().Interface()
	}
	dst := reflect.New(typ)
	return dst.Interface()
}

// ToReflectValues convert the go objects into reflect.Value slice
func ToReflectValues(args ...interface{}) (values []reflect.Value) {
	for i := range args {
		arg := args[i]
		values = append(values, reflect.ValueOf(arg))
	}

	return values
}

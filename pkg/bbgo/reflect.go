package bbgo

import (
	"errors"
	"fmt"
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

func hasField(rs reflect.Value, fieldName string) (field reflect.Value, ok bool) {
	field = rs.FieldByName(fieldName)
	return field, field.IsValid()
}

type StructFieldIterator func(tag string, ft reflect.StructField, fv reflect.Value) error

var errCanNotIterateNilPointer = errors.New("can not iterate struct on a nil pointer")

func iterateFieldsByTag(obj interface{}, tagName string, cb StructFieldIterator) error {
	sv := reflect.ValueOf(obj)
	st := reflect.TypeOf(obj)

	if st.Kind() != reflect.Ptr {
		return fmt.Errorf("f should be a pointer of a struct, %s given", st)
	}

	// for pointer, check if it's nil
	if sv.IsNil() {
		return errCanNotIterateNilPointer
	}

	// solve the reference
	st = st.Elem()
	sv = sv.Elem()

	if st.Kind() != reflect.Struct {
		return fmt.Errorf("f should be a struct, %s given", st)
	}

	for i := 0; i < sv.NumField(); i++ {
		fv := sv.Field(i)
		ft := st.Field(i)

		// skip unexported fields
		if !st.Field(i).IsExported() {
			continue
		}

		tag, ok := ft.Tag.Lookup(tagName)
		if !ok {
			continue
		}

		if err := cb(tag, ft, fv); err != nil {
			return err
		}
	}

	return nil
}

// https://github.com/xiaojun207/go-base-utils/blob/master/utils/Clone.go
func newTypeValueInterface(typ reflect.Type) interface{} {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		dst := reflect.New(typ).Elem()
		return dst.Addr().Interface()
	}
	dst := reflect.New(typ)
	return dst.Interface()
}

// toReflectValues convert the go objects into reflect.Value slice
func toReflectValues(args ...interface{}) (values []reflect.Value) {
	for _, arg := range args {
		values = append(values, reflect.ValueOf(arg))
	}

	return values
}

func reflectMergeStructFields(dst, src interface{}) {
	rtA := reflect.TypeOf(dst)
	srcStructType := reflect.TypeOf(src)

	rtA = rtA.Elem()
	srcStructType = srcStructType.Elem()

	for i := 0; i < rtA.NumField(); i++ {
		fieldType := rtA.Field(i)
		fieldName := fieldType.Name
		if fieldSrcType, ok := srcStructType.FieldByName(fieldName); ok {
			if fieldSrcType.Type == fieldType.Type {
				srcValue := reflect.ValueOf(src).Elem().FieldByName(fieldName)
				dstValue := reflect.ValueOf(dst).Elem().FieldByName(fieldName)
				if (fieldType.Type.Kind() == reflect.Ptr && dstValue.IsNil()) || dstValue.IsZero() {
					dstValue.Set(srcValue)
				}
			}
		}
	}
}

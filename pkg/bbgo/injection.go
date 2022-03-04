package bbgo

import (
	"fmt"
	"reflect"

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

func parseStructAndInject(f interface{}, objects ...interface{}) error {
	sv := reflect.ValueOf(f)
	st := reflect.TypeOf(f)

	if st.Kind() != reflect.Ptr {
		return fmt.Errorf("f needs to be a pointer of a struct, %s given", st)
	}

	// solve the pointer
	st = st.Elem()
	sv = sv.Elem()

	if st.Kind() != reflect.Struct {
		return fmt.Errorf("f needs to be a struct, %s given", st)
	}

	for i := 0; i < sv.NumField(); i++ {
		fv := sv.Field(i)
		ft := fv.Type()

		// skip unexported fields
		if !st.Field(i).IsExported() {
			continue
		}

		switch k := fv.Kind(); k {

		case reflect.Ptr, reflect.Struct:
			for oi := 0; oi < len(objects); oi++ {
				obj := objects[oi]
				if obj == nil {
					continue
				}

				ot := reflect.TypeOf(obj)
				if ft.AssignableTo(ot) {
					if !fv.CanSet() {
						return fmt.Errorf("field %v of %s can not be set to %s, make sure it is an exported field", fv, sv.Type(), ot)
					}
					fv.Set(reflect.ValueOf(obj))
				}
			}

		case reflect.Interface:
			for oi := 0; oi < len(objects); oi++ {
				obj := objects[oi]
				if obj == nil {
					continue
				}

				objT := reflect.TypeOf(obj)
				logrus.Debugln(
					ft.PkgPath(),
					ft.Name(),
					objT, "implements", ft, "=", objT.Implements(ft),
				)

				if objT.Implements(ft) {
					fv.Set(reflect.ValueOf(obj))
				}
			}
		}
	}

	return nil
}

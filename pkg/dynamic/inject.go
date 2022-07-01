package dynamic

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

type testEnvironment struct {
	startTime time.Time
}

func InjectField(rs reflect.Value, fieldName string, obj interface{}, pointerOnly bool) error {
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

// ParseStructAndInject parses the struct fields and injects the objects into the corresponding fields by its type.
// if the given object is a reference of an object, the type of the target field MUST BE a pointer field.
// if the given object is a struct value, the type of the target field CAN BE a pointer field or a struct value field.
func ParseStructAndInject(f interface{}, objects ...interface{}) error {
	sv := reflect.ValueOf(f)
	st := reflect.TypeOf(f)

	if st.Kind() != reflect.Ptr {
		return fmt.Errorf("f needs to be a pointer of a struct, %s given", st)
	}

	// solve the reference
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

		fieldName := st.Field(i).Name

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

					if k == reflect.Ptr && !fv.IsNil() {
						logrus.Debugf("[injection] field %s is not nil, not injecting", fieldName)
						continue
					}

					if k == reflect.Ptr && ot.Kind() == reflect.Struct {
						logrus.Debugf("[injection] found ptr + struct, injecting field %s to %T", fieldName, obj)
						fv.Set(reflect.ValueOf(obj).Addr())
					} else {
						logrus.Debugf("[injection] injecting field %s to %T", fieldName, obj)
						fv.Set(reflect.ValueOf(obj))
					}
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

func Test_injectField(t *testing.T) {
	type TT struct {
		TradeService *service.TradeService
	}

	// only pointer object can be set.
	var tt = &TT{}

	// get the value of the pointer, or it can not be set.
	var rv = reflect.ValueOf(tt).Elem()

	_, ret := HasField(rv, "TradeService")
	assert.True(t, ret)

	ts := &service.TradeService{}

	err := InjectField(rv, "TradeService", ts, true)
	assert.NoError(t, err)
}

func Test_parseStructAndInject(t *testing.T) {
	t.Run("skip nil", func(t *testing.T) {
		ss := struct {
			a   int
			Env *testEnvironment
		}{
			a:   1,
			Env: nil,
		}
		err := ParseStructAndInject(&ss, nil)
		assert.NoError(t, err)
		assert.Nil(t, ss.Env)
	})
	t.Run("pointer", func(t *testing.T) {
		ss := struct {
			a   int
			Env *testEnvironment
		}{
			a:   1,
			Env: nil,
		}
		err := ParseStructAndInject(&ss, &testEnvironment{})
		assert.NoError(t, err)
		assert.NotNil(t, ss.Env)
	})

	t.Run("composition", func(t *testing.T) {
		type TT struct {
			*service.TradeService
		}
		ss := TT{}
		err := ParseStructAndInject(&ss, &service.TradeService{})
		assert.NoError(t, err)
		assert.NotNil(t, ss.TradeService)
	})

	t.Run("struct", func(t *testing.T) {
		ss := struct {
			a   int
			Env testEnvironment
		}{
			a: 1,
		}
		err := ParseStructAndInject(&ss, testEnvironment{
			startTime: time.Now(),
		})
		assert.NoError(t, err)
		assert.NotEqual(t, time.Time{}, ss.Env.startTime)
	})
	t.Run("interface/any", func(t *testing.T) {
		ss := struct {
			Any interface{} // anything
		}{
			Any: nil,
		}
		err := ParseStructAndInject(&ss, &testEnvironment{
			startTime: time.Now(),
		})
		assert.NoError(t, err)
		assert.NotNil(t, ss.Any)
	})
	t.Run("interface/stringer", func(t *testing.T) {
		ss := struct {
			Stringer types.Stringer // stringer interface
		}{
			Stringer: nil,
		}
		err := ParseStructAndInject(&ss, &types.Trade{})
		assert.NoError(t, err)
		assert.NotNil(t, ss.Stringer)
	})
}

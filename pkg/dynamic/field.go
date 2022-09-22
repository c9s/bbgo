package dynamic

import (
	"errors"
	"reflect"
	"strings"
)

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

// Used by bbgo/interact_modify.go
func GetModifiableFields(val reflect.Value, callback func(tagName, name string)) {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if !val.IsValid() {
		return
	}
	num := val.Type().NumField()
	for i := 0; i < num; i++ {
		t := val.Type().Field(i)
		if !t.IsExported() {
			continue
		}
		if t.Anonymous {
			GetModifiableFields(val.Field(i), callback)
		}
		modifiable := t.Tag.Get("modifiable")
		if modifiable != "true" {
			continue
		}
		jsonTag := t.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name := strings.Split(jsonTag, ",")[0]
		callback(name, t.Name)
	}
}

var zeroValue reflect.Value = reflect.Zero(reflect.TypeOf(0))

func GetModifiableField(val reflect.Value, name string) (reflect.Value, bool) {
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return zeroValue, false
		}
	}
	if val.Kind() != reflect.Struct {
		return zeroValue, false
	}
	if !val.IsValid() {
		return zeroValue, false
	}
	field, ok := val.Type().FieldByName(name)
	if !ok {
		return zeroValue, ok
	}
	if field.Tag.Get("modifiable") != "true" {
		return zeroValue, false
	}
	jsonTag := field.Tag.Get("json")
	if jsonTag == "" || jsonTag == "-" {
		return zeroValue, false
	}
	value, err := FieldByIndexErr(val, field.Index)
	if err != nil {
		return zeroValue, false
	}
	return value, true
}

// Modified from golang 1.19.1 reflect to eliminate all possible panic
func FieldByIndexErr(v reflect.Value, index []int) (reflect.Value, error) {
	if len(index) == 1 {
		return v.Field(index[0]), nil
	}
	if v.Kind() != reflect.Struct {
		return zeroValue, errors.New("should receive a Struct")
	}
	for i, x := range index {
		if i > 0 {
			if v.Kind() == reflect.Ptr && v.Type().Elem().Kind() == reflect.Struct {
				if v.IsNil() {
					return zeroValue, errors.New("reflect: indirection through nil pointer to embedded struct field ")
				}
				v = v.Elem()
			}
		}
		v = v.Field(x)
	}
	return v, nil
}

package dynamic

import (
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
	for i := 0; i < val.Type().NumField(); i++ {
		t := val.Type().Field(i)
		if !t.IsExported() {
			continue
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
	return val.FieldByName(name), true
}
